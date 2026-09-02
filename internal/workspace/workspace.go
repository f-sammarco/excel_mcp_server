// Package workspace resolves the file paths that tool arguments carry into paths
// on the machine the server runs on.
//
// Over stdio the client and the server share a filesystem, so an absolute path
// means the same thing on both sides. Over HTTP they do not: the client has no
// way to know which paths exist on the server, and letting it name any of them
// hands a remote caller the server's whole disk. Both problems are solved by a
// workspace directory — a temporary folder the server owns. Relative paths
// resolve inside it, and in restricted mode nothing outside it can be touched,
// so a caller can create a workbook without knowing anything about the server's
// filesystem and then take the result back as an attachment.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// EnvDir overrides the workspace directory.
	EnvDir = "EXCEL_MCP_WORKSPACE_DIR"
	// EnvRestrict forces the restricted mode on ("true") or off ("false"),
	// overriding the per-transport default.
	EnvRestrict = "EXCEL_MCP_RESTRICT_TO_WORKSPACE"
	// EnvTransport is read only to pick the default for EnvRestrict.
	EnvTransport = "EXCEL_MCP_TRANSPORT"

	defaultDirName = "excel-mcp-server"
)

var (
	dirOnce sync.Once
	dirPath string
)

// Dir returns the workspace directory, creating it if needed. The directory is
// resolved once per process so a relative or symlinked configuration keeps
// pointing at the same place for the life of the server.
func Dir() string {
	dirOnce.Do(func() {
		dir := strings.TrimSpace(os.Getenv(EnvDir))
		if dir == "" {
			dir = filepath.Join(os.TempDir(), defaultDirName)
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		// A failure here is not fatal: Resolve reports it when a path is actually
		// used, which is a better place to surface it than server startup.
		_ = os.MkdirAll(dir, 0o700)
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		dirPath = filepath.Clean(dir)
	})
	return dirPath
}

// Restricted reports whether paths outside the workspace directory are refused.
// It defaults to true for the HTTP transport, where the caller is remote, and
// false for stdio, where the client already shares the filesystem.
func Restricted() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvRestrict))) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	return strings.ToLower(strings.TrimSpace(os.Getenv(EnvTransport))) == "http"
}

// Resolve turns a tool argument into a path on this machine.
//
// A relative path is resolved inside the workspace directory and its parent
// directories are created, so a caller that does not know the server's
// filesystem can still name a destination. An absolute path is returned as
// given, unless the server is restricted to the workspace, in which case it must
// already point inside it.
func Resolve(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("file path is empty")
	}

	if !filepath.IsAbs(trimmed) {
		resolved := filepath.Clean(filepath.Join(Dir(), trimmed))
		if !within(Dir(), resolved) {
			return "", fmt.Errorf("path '%s' escapes the workspace directory", path)
		}
		if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
			return "", err
		}
		return resolved, nil
	}

	resolved := realPath(filepath.Clean(trimmed))
	if Restricted() {
		if !within(Dir(), resolved) {
			return "", fmt.Errorf(
				"path '%s' is outside the workspace directory '%s'; this server only accepts paths inside it, so pass a path relative to the workspace instead",
				path, Dir(),
			)
		}
		if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
			return "", err
		}
	}
	return resolved, nil
}

// realPath resolves symlinks in the longest existing prefix of path and
// re-appends the rest. Dir() is symlink-resolved, so an absolute argument has to
// be too or a workspace under a symlinked temp directory (/tmp on macOS) would
// never look like it is inside itself.
func realPath(path string) string {
	remainder := ""
	current := path
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Clean(filepath.Join(resolved, remainder))
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
}

// within reports whether path is root itself or sits under it. It compares
// cleaned paths, so "/tmp/ws-other" does not count as being under "/tmp/ws".
func within(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// resetForTest clears the memoized workspace directory so a test can point the
// package at a different one.
func resetForTest() {
	dirOnce = sync.Once{}
	dirPath = ""
}

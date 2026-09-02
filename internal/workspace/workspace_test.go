package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

// setup points the package at a fresh workspace directory. The directory is
// resolved once per process, so the sync.Once has to be reset with it.
func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(EnvDir, dir)
	t.Setenv(EnvRestrict, "")
	t.Setenv(EnvTransport, "")
	resetForTest()
	t.Cleanup(resetForTest)
	return Dir()
}

func TestResolveRelativePathLandsInWorkspace(t *testing.T) {
	root := setup(t)

	resolved, err := Resolve("reports/q1.xlsx")
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}
	if want := filepath.Join(root, "reports", "q1.xlsx"); resolved != want {
		t.Errorf("resolved = %q, want %q", resolved, want)
	}
	// The parent directory has to exist for a workbook to be saved into it.
	if _, err := filepath.Abs(resolved); err != nil {
		t.Fatalf("resolved path is not usable: %v", err)
	}
}

func TestResolveRelativePathCannotEscape(t *testing.T) {
	setup(t)

	if _, err := Resolve("../outside.xlsx"); err == nil {
		t.Fatal("Resolve accepted a path escaping the workspace")
	}
}

func TestResolveAbsolutePathAllowedWhenUnrestricted(t *testing.T) {
	setup(t)
	t.Setenv(EnvRestrict, "false")

	outside := filepath.Join(t.TempDir(), "book.xlsx")
	resolved, err := Resolve(outside)
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}
	if resolved != outside {
		t.Errorf("resolved = %q, want %q", resolved, outside)
	}
}

func TestResolveAbsolutePathRefusedWhenRestricted(t *testing.T) {
	setup(t)
	t.Setenv(EnvRestrict, "true")

	outside := filepath.Join(t.TempDir(), "book.xlsx")
	_, err := Resolve(outside)
	if err == nil {
		t.Fatal("Resolve accepted a path outside the workspace while restricted")
	}
	if !strings.Contains(err.Error(), "outside the workspace") {
		t.Errorf("error = %q, want it to explain the workspace restriction", err)
	}
}

func TestResolveAbsolutePathInsideWorkspaceAllowedWhenRestricted(t *testing.T) {
	root := setup(t)
	t.Setenv(EnvRestrict, "true")

	inside := filepath.Join(root, "sub", "book.xlsx")
	resolved, err := Resolve(inside)
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}
	if resolved != inside {
		t.Errorf("resolved = %q, want %q", resolved, inside)
	}
}

func TestResolveRejectsEmptyPath(t *testing.T) {
	setup(t)

	if _, err := Resolve("   "); err == nil {
		t.Fatal("Resolve accepted an empty path")
	}
}

func TestRestrictedDefaultsToTransport(t *testing.T) {
	setup(t)

	t.Setenv(EnvTransport, "http")
	if !Restricted() {
		t.Error("Restricted() = false for the http transport, want true")
	}
	t.Setenv(EnvTransport, "stdio")
	if Restricted() {
		t.Error("Restricted() = true for the stdio transport, want false")
	}
}

// A sibling directory sharing the workspace's name prefix is outside it.
func TestSiblingPrefixIsNotWithinWorkspace(t *testing.T) {
	root := setup(t)
	t.Setenv(EnvRestrict, "true")

	if _, err := Resolve(root + "-other/book.xlsx"); err == nil {
		t.Fatal("Resolve accepted a sibling directory sharing the workspace name prefix")
	}
}

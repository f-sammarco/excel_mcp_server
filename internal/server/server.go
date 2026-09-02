package server

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	"github.com/negokaz/excel-mcp-server/internal/tools"
	"github.com/negokaz/excel-mcp-server/internal/workspace"
)

const (
	// EnvTransport selects the transport: "stdio" (default) or "http".
	EnvTransport = "EXCEL_MCP_TRANSPORT"
	// EnvHTTPAddr is the address the HTTP transport listens on.
	EnvHTTPAddr = "EXCEL_MCP_HTTP_ADDR"
	// EnvHTTPPath is the endpoint path the HTTP transport serves.
	EnvHTTPPath = "EXCEL_MCP_HTTP_PATH"
	// EnvHTTPStateless serves every request without a session when true.
	EnvHTTPStateless = "EXCEL_MCP_HTTP_STATELESS"

	defaultHTTPAddr = "localhost:8000"
	defaultHTTPPath = "/mcp"
)

type ExcelServer struct {
	server *server.MCPServer
}

func New(version string) *ExcelServer {
	s := &ExcelServer{}
	s.server = server.NewMCPServer(
		"excel-mcp-server",
		version,
	)
	tools.AddExcelDescribeSheetsTool(s.server)
	tools.AddExcelReadSheetTool(s.server)
	if runtime.GOOS == "windows" {
		tools.AddExcelScreenCaptureTool(s.server)
	}
	tools.AddExcelWriteToSheetTool(s.server)
	tools.AddExcelCreateTableTool(s.server)
	tools.AddExcelCopySheetTool(s.server)
	tools.AddExcelFormatRangeTool(s.server)
	tools.AddExcelExportFileTool(s.server)
	return s
}

func (s *ExcelServer) Start() error {
	switch transport := strings.ToLower(strings.TrimSpace(os.Getenv(EnvTransport))); transport {
	case "", "stdio":
		return server.ServeStdio(s.server)
	case "http", "streamable-http", "streamable_http":
		return s.startStreamableHTTP()
	default:
		return fmt.Errorf("unknown transport %q: expected \"stdio\" or \"http\"", transport)
	}
}

// startStreamableHTTP serves the MCP Streamable HTTP transport: one endpoint
// handling POST for requests, GET for the server-to-client SSE stream, and
// DELETE to end a session.
//
// The transport carries no authentication of its own, so the default address is
// loopback-only. Exposing it beyond the host means putting an authenticating
// proxy in front of it.
func (s *ExcelServer) startStreamableHTTP() error {
	addr := strings.TrimSpace(os.Getenv(EnvHTTPAddr))
	if addr == "" {
		addr = defaultHTTPAddr
	}
	path := strings.TrimSpace(os.Getenv(EnvHTTPPath))
	if path == "" {
		path = defaultHTTPPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	options := []server.StreamableHTTPOption{server.WithEndpointPath(path)}
	if isTrue(os.Getenv(EnvHTTPStateless)) {
		options = append(options, server.WithStateLess(true))
	}

	httpServer := server.NewStreamableHTTPServer(s.server, options...)
	fmt.Fprintf(os.Stderr, "excel-mcp-server listening on http://%s%s (workspace: %s, restricted: %t)\n",
		addr, path, workspace.Dir(), workspace.Restricted())
	return httpServer.Start(addr)
}

func isTrue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		return true
	}
	return false
}

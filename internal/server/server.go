package server

import (
	"fmt"
	"net/http"
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
	// EnvHTTPHealthPath is the path serving the liveness/readiness endpoint.
	EnvHTTPHealthPath = "EXCEL_MCP_HTTP_HEALTH_PATH"
	// EnvHTTPStateless serves every request without a session when true.
	EnvHTTPStateless = "EXCEL_MCP_HTTP_STATELESS"

	defaultHTTPAddr       = "localhost:8000"
	defaultHTTPPath       = "/mcp"
	defaultHTTPHealthPath = "/healthz"
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
	path := normalizePath(os.Getenv(EnvHTTPPath), defaultHTTPPath)

	options := []server.StreamableHTTPOption{server.WithEndpointPath(path)}
	if isTrue(os.Getenv(EnvHTTPStateless)) {
		options = append(options, server.WithStateLess(true))
	}

	healthPath := normalizePath(os.Getenv(EnvHTTPHealthPath), defaultHTTPHealthPath)

	httpServer := server.NewStreamableHTTPServer(s.server, options...)

	// The MCP endpoint rejects a bare GET (it expects a POST, or a GET carrying a
	// session), so it cannot double as a health check: an orchestrator probing it
	// reads the 4xx as a dead container and restarts the pod. Serve a separate
	// endpoint that only reports the process is listening.
	mux := http.NewServeMux()
	// Registering the same pattern twice panics, and the MCP endpoint wins the
	// collision: it is the one the transport cannot do without.
	if healthPath == path {
		return fmt.Errorf("%s and %s must differ (both are %q)", EnvHTTPHealthPath, EnvHTTPPath, path)
	}
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle(path, httpServer)

	fmt.Fprintf(os.Stderr, "excel-mcp-server listening on http://%s%s (health: %s, workspace: %s, restricted: %t)\n",
		addr, path, healthPath, workspace.Dir(), workspace.Restricted())
	return http.ListenAndServe(addr, mux)
}

// normalizePath falls back to fallback when value is blank and makes the result
// rooted, since http.ServeMux only accepts patterns starting with "/".
func normalizePath(value string, fallback string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		path = fallback
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func isTrue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		return true
	}
	return false
}

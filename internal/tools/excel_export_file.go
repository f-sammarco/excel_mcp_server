package tools

import (
	"context"

	z "github.com/Oudwins/zog"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	imcp "github.com/negokaz/excel-mcp-server/internal/mcp"
)

type ExcelExportFileArguments struct {
	FileAbsolutePath string `zog:"fileAbsolutePath"`
}

var excelExportFileArgumentsSchema = z.Struct(z.Shape{
	"fileAbsolutePath": z.String().Test(FilePathTest()).Required(),
})

// AddExcelExportFileTool registers the tool that ends a workbook-building
// conversation.
//
// Every other tool works on the server's own filesystem, which the caller may not
// share: the workbook it just built exists only inside the workspace directory. This
// tool is the way back out — it reads the saved file and returns it as an embedded
// blob resource, so the caller can download it.
//
// It is a separate call rather than a flag on a write because a workbook is finished
// by whatever tool happens to come last — a format, a table, a copy — and no writer
// can know it was the final one. Exporting explicitly also guarantees the bytes
// handed over include every later edit.
func AddExcelExportFileTool(server *server.MCPServer) {
	server.AddTool(mcp.NewTool("excel_export_file",
		mcp.WithDescription("Export a saved Excel file as a downloadable binary attachment (base64 blob resource). Call this once, as the last step, after every edit to a workbook the caller must be able to download: the file otherwise stays on the server's filesystem and the caller never receives it. The file must be 20 MB or smaller."),
		mcp.WithString("fileAbsolutePath",
			mcp.Required(),
			mcp.Description("Absolute path to the Excel file, or a path relative to the server workspace directory. A relative path is the one to use when the server does not share a filesystem with you."),
		),
	), handleExportFile)
}

func handleExportFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := ExcelExportFileArguments{}
	issues := excelExportFileArgumentsSchema.Parse(request.Params.Arguments, &args)
	if len(issues) != 0 {
		return imcp.NewToolResultZogIssueMap(issues), nil
	}
	filePath, errResult := ResolveFilePath(args.FileAbsolutePath)
	if errResult != nil {
		return errResult, nil
	}
	return exportFile(filePath)
}

func exportFile(fileAbsolutePath string) (*mcp.CallToolResult, error) {
	html := "<h2>Metadata</h2>\n<ul>\n"
	html += "<li>file path: " + fileAbsolutePath + "</li>\n"
	html += "</ul>\n"
	return AttachWorkbookFile(mcp.NewToolResultText(html), fileAbsolutePath), nil
}

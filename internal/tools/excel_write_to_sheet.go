package tools

import (
	"context"
	"fmt"

	z "github.com/Oudwins/zog"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/negokaz/excel-mcp-server/internal/excel"
	imcp "github.com/negokaz/excel-mcp-server/internal/mcp"
	"github.com/xuri/excelize/v2"
)

type ExcelWriteToSheetArguments struct {
	FileAbsolutePath string     `zog:"fileAbsolutePath"`
	SheetName        string     `zog:"sheetName"`
	NewSheet         bool       `zog:"newSheet"`
	Range            string     `zog:"range"`
	Values           [][]string `zog:"values"`
	AttachFile       bool       `zog:"attachFile"`
}

var excelWriteToSheetArgumentsSchema = z.Struct(z.Shape{
	"fileAbsolutePath": z.String().Test(FilePathTest()).Required(),
	"sheetName":        z.String().Required(),
	"newSheet":         z.Bool().Required().Default(false),
	"range":            z.String().Required(),
	"values":           z.Slice(z.Slice(z.String())).Required(),
	"attachFile":       z.Bool().Default(false),
})

func AddExcelWriteToSheetTool(server *server.MCPServer) {
	server.AddTool(mcp.NewTool("excel_write_to_sheet",
		mcp.WithDescription("Write values to the Excel sheet. The workbook is created if the file does not exist yet, so a relative path plus attachFile builds a new workbook from scratch and returns it as a download."),
		mcp.WithString("fileAbsolutePath",
			mcp.Required(),
			mcp.Description("Absolute path to the Excel file, or a path relative to the server workspace directory. A relative path is the one to use when the server does not share a filesystem with you."),
		),
		mcp.WithString("sheetName",
			mcp.Required(),
			mcp.Description("Sheet name in the Excel file"),
		),
		mcp.WithBoolean("newSheet",
			mcp.Required(),
			mcp.Description("Create a new sheet if true, otherwise write to the existing sheet"),
		),
		mcp.WithString("range",
			mcp.Required(),
			mcp.Description("Range of cells in the Excel sheet (e.g., \"A1:C10\")"),
		),
		mcp.WithArray("values",
			mcp.Required(),
			mcp.Description("Values to write to the Excel sheet. If the value is a formula, it should start with \"=\""),
			mcp.Items(map[string]any{
				"type": "array",
				"items": map[string]any{
					"anyOf": []any{
						map[string]any{
							"type": "string",
						},
						map[string]any{
							"type": "number",
						},
						map[string]any{
							"type": "boolean",
						},
						map[string]any{
							"type": "null",
						},
					},
				},
			}),
		),
		mcp.WithBoolean("attachFile",
			mcp.Description("Attach the saved workbook to the result as a downloadable binary attachment (base64 blob resource). Set true on the last write of a file the caller must be able to download; the file must be 20 MB or smaller."),
		),
	), handleWriteToSheet)
}

func handleWriteToSheet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := ExcelWriteToSheetArguments{}
	issues := excelWriteToSheetArgumentsSchema.Parse(request.Params.Arguments, &args)
	if len(issues) != 0 {
		return imcp.NewToolResultZogIssueMap(issues), nil
	}

	// zog が any type のスキーマをサポートしていないため、自力で実装
	valuesArg, ok := request.GetArguments()["values"].([]any)
	if !ok {
		return imcp.NewToolResultInvalidArgumentError("values must be a 2D array"), nil
	}
	values := make([][]any, len(valuesArg))
	for i, v := range valuesArg {
		value, ok := v.([]any)
		if !ok {
			return imcp.NewToolResultInvalidArgumentError("values must be a 2D array"), nil
		}
		values[i] = value
	}

	filePath, errResult := ResolveFilePath(args.FileAbsolutePath)
	if errResult != nil {
		return errResult, nil
	}
	return writeSheet(filePath, args.SheetName, args.NewSheet, args.Range, values, args.AttachFile)
}

func writeSheet(fileAbsolutePath string, sheetName string, newSheet bool, rangeStr string, values [][]any, attachFile bool) (*mcp.CallToolResult, error) {
	// A missing file is created rather than refused: writing is how a workbook
	// gets built from scratch, and with a workspace-relative path the caller has
	// no other way to bring one into existence.
	workbook, closeFn, created, err := excel.OpenOrCreateFile(fileAbsolutePath, sheetName)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	startCol, startRow, endCol, endRow, err := excel.ParseRange(rangeStr)
	if err != nil {
		return imcp.NewToolResultInvalidArgumentError(err.Error()), nil
	}

	// データの整合性チェック
	rangeRowSize := endRow - startRow + 1
	if len(values) != rangeRowSize {
		return imcp.NewToolResultInvalidArgumentError(fmt.Sprintf("number of rows in data (%d) does not match range size (%d)", len(values), rangeRowSize)), nil
	}

	// A file created just now already holds sheetName as its only sheet.
	if newSheet && !created {
		if err := workbook.CreateNewSheet(sheetName); err != nil {
			return nil, err
		}
	}

	// シートの取得
	worksheet, err := workbook.FindSheet(sheetName)
	if err != nil {
		return imcp.NewToolResultInvalidArgumentError(err.Error()), nil
	}
	defer worksheet.Release()

	// データの書き込み
	wroteFormula := false
	for i, row := range values {
		rangeColumnSize := endCol - startCol + 1
		if len(row) != rangeColumnSize {
			return imcp.NewToolResultInvalidArgumentError(fmt.Sprintf("number of columns in row %d (%d) does not match range size (%d)", i, len(row), rangeColumnSize)), nil
		}
		for j, cellValue := range row {
			cell, err := excelize.CoordinatesToCellName(startCol+j, startRow+i)
			if err != nil {
				return nil, err
			}
			if cellStr, ok := cellValue.(string); ok && isFormula(cellStr) {
				// if cellValue is formula, set it as formula
				err = worksheet.SetFormula(cell, cellStr)
				wroteFormula = true
			} else {
				// if cellValue is not formula, set it as value
				err = worksheet.SetValue(cell, cellValue)
			}
			if err != nil {
				return nil, err
			}
		}
	}

	if err := workbook.Save(); err != nil {
		return nil, err
	}

	// HTMLテーブルの生成
	var table *string
	if wroteFormula {
		table, err = CreateHTMLTableOfFormula(worksheet, startCol, startRow, endCol, endRow)
	} else {
		table, err = CreateHTMLTableOfValues(worksheet, startCol, startRow, endCol, endRow)
	}
	if err != nil {
		return nil, err
	}
	html := "<h2>Written Sheet</h2>\n"
	html += *table + "\n"
	html += "<h2>Metadata</h2>\n"
	html += "<ul>\n"
	html += fmt.Sprintf("<li>backend: %s</li>\n", workbook.GetBackendName())
	html += fmt.Sprintf("<li>file path: %s</li>\n", fileAbsolutePath)
	html += fmt.Sprintf("<li>sheet name: %s</li>\n", sheetName)
	html += fmt.Sprintf("<li>read range: %s</li>\n", rangeStr)
	html += "</ul>\n"
	html += "<h2>Notice</h2>\n"
	html += "<p>Values wrote successfully.</p>\n"

	result := mcp.NewToolResultText(html)
	if attachFile {
		result = AttachWorkbookFile(result, fileAbsolutePath)
	}
	return result, nil
}

func isFormula(value string) bool {
	return len(value) > 0 && value[0] == '='
}

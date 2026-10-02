package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/xuri/excelize/v2"
)

func TestWriteHandlerAcceptsAdvertisedCellTypes(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "mixed.xlsx")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"fileAbsolutePath": filePath,
		"sheetName":        "Data",
		"newSheet":         false,
		"range":            "A1:E1",
	}
	var values any
	if err := json.Unmarshal([]byte(`[["text",42.5,true,null,"=B1*2"]]`), &values); err != nil {
		t.Fatal(err)
	}
	request.GetArguments()["values"] = values
	result, err := handleWriteToSheet(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("mixed cell types were rejected: %#v", result.Content)
	}
	workbook, err := excelize.OpenFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	for cell, expected := range map[string]string{"A1": "text", "B1": "42.5", "C1": "TRUE", "D1": ""} {
		actual, err := workbook.GetCellValue("Data", cell)
		if err != nil || actual != expected {
			t.Fatalf("%s = %q (%v), expected %q", cell, actual, err, expected)
		}
	}
	formula, err := workbook.GetCellFormula("Data", "E1")
	if err != nil || formula != "=B1*2" {
		t.Fatalf("formula = %q (%v)", formula, err)
	}
	for _, invalid := range []string{`[[{}]]`, `[[[]]]`, `["text"]`, `null`} {
		if err := json.Unmarshal([]byte(invalid), &values); err != nil {
			t.Fatal(err)
		}
		invalidPath := filepath.Join(t.TempDir(), "invalid.xlsx")
		request.GetArguments()["fileAbsolutePath"] = invalidPath
		request.GetArguments()["range"] = "A1"
		request.GetArguments()["values"] = values
		result, err := handleWriteToSheet(context.Background(), request)
		if err != nil || !result.IsError {
			t.Fatalf("invalid values %s were not rejected: %v, %v", invalid, result, err)
		}
		if _, err := os.Stat(invalidPath); !os.IsNotExist(err) {
			t.Fatalf("invalid values created a workbook: %v", err)
		}
	}
}

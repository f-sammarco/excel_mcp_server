package tools

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/xuri/excelize/v2"
)

func newWorkbook(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f := excelize.NewFile()
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func findBlob(content []mcp.Content) *mcp.BlobResourceContents {
	for _, item := range content {
		if embedded, ok := item.(mcp.EmbeddedResource); ok {
			if blob, ok := embedded.Resource.(mcp.BlobResourceContents); ok {
				return &blob
			}
		}
	}
	return nil
}

func TestExportFileAttachesTheWorkbook(t *testing.T) {
	path := newWorkbook(t, "book.xlsx")
	result, err := exportFile(path)
	if err != nil {
		t.Fatal(err)
	}
	blob := findBlob(result.Content)
	if blob == nil {
		t.Fatalf("no blob resource in result: %#v", result.Content)
	}
	if blob.MIMEType != attachmentMimeTypes[".xlsx"] {
		t.Errorf("unexpected MIME type: %s", blob.MIMEType)
	}
	decoded, err := base64.StdEncoding.DecodeString(blob.Blob)
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(onDisk) {
		t.Errorf("blob is %d bytes, file is %d bytes", len(decoded), len(onDisk))
	}
}

func TestWriteSheetDoesNotAttachTheWorkbook(t *testing.T) {
	path := newWorkbook(t, "book.xlsx")
	result, err := writeSheet(path, "Sheet1", false, "A1:B1", [][]any{{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if blob := findBlob(result.Content); blob != nil {
		t.Error("a write attached a blob; only excel_export_file may")
	}
}

func TestExportFileSeesEveryEditMadeBeforeIt(t *testing.T) {
	path := newWorkbook(t, "book.xlsx")
	if _, err := writeSheet(path, "Sheet1", false, "A1:B1", [][]any{{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	result, err := exportFile(path)
	if err != nil {
		t.Fatal(err)
	}
	blob := findBlob(result.Content)
	if blob == nil {
		t.Fatalf("no blob resource in result: %#v", result.Content)
	}
	decoded, err := base64.StdEncoding.DecodeString(blob.Blob)
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(onDisk) {
		t.Errorf("blob is %d bytes, the file on disk is %d bytes", len(decoded), len(onDisk))
	}
}

func TestExportFileFailsWhenItCannotAttach(t *testing.T) {
	result := AttachWorkbookFile(mcp.NewToolResultText("exported"), "/tmp/data.csv")
	if findBlob(result.Content) != nil {
		t.Error("a non-Excel extension was attached")
	}
	if !result.IsError {
		t.Error("an export that attached nothing must be an error, not a silent success")
	}
}

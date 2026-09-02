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

func TestWriteSheetAttachesTheWorkbook(t *testing.T) {
	path := newWorkbook(t, "book.xlsx")
	result, err := writeSheet(path, "Sheet1", false, "A1:B1", [][]any{{"a", "b"}}, true)
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

func TestWriteSheetWithoutAttachFileReturnsTextOnly(t *testing.T) {
	path := newWorkbook(t, "book.xlsx")
	result, err := writeSheet(path, "Sheet1", false, "A1:B1", [][]any{{"a", "b"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if blob := findBlob(result.Content); blob != nil {
		t.Error("a blob was attached even though attachFile was false")
	}
}

func TestAttachWorkbookFileKeepsTheResultWhenItCannotAttach(t *testing.T) {
	result := AttachWorkbookFile(mcp.NewToolResultText("written"), "/tmp/data.csv")
	if findBlob(result.Content) != nil {
		t.Error("a non-Excel extension was attached")
	}
	if result.IsError {
		t.Error("a failed attachment must not turn a successful write into an error")
	}
	if len(result.Content) < 2 {
		t.Error("expected a notice explaining why nothing was attached")
	}
}

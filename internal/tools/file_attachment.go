package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Excel files this server can produce, mapped to the MIME type a client needs to
// hand the bytes to the OS as a file rather than render them.
var attachmentMimeTypes = map[string]string{
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".xlsm": "application/vnd.ms-excel.sheet.macroEnabled.12",
	".xltx": "application/vnd.openxmlformats-officedocument.spreadsheetml.template",
	".xltm": "application/vnd.ms-excel.template.macroEnabled.12",
}

// A base64 blob rides inside the JSON-RPC response, so the whole workbook sits in one
// message. Past this size a client is more likely to choke than to deliver it.
const attachmentMaxBytes = 20 * 1024 * 1024

// AttachWorkbookFile appends the saved workbook to a tool result as an embedded blob
// resource, so the caller can offer it as a download instead of a local path. The
// workbook must already be saved: the bytes come from disk.
//
// A failure to attach never discards the result — the write itself succeeded, and the
// caller needs to know that more than it needs the file. The reason is appended as
// text instead.
func AttachWorkbookFile(result *mcp.CallToolResult, fileAbsolutePath string) *mcp.CallToolResult {
	ext := strings.ToLower(filepath.Ext(fileAbsolutePath))
	mimeType, ok := attachmentMimeTypes[ext]
	if !ok {
		return appendNotice(result, fmt.Sprintf("File not attached: unsupported file extension: %s", ext))
	}

	info, err := os.Stat(fileAbsolutePath)
	if err != nil {
		return appendNotice(result, fmt.Sprintf("File not attached: %s", err.Error()))
	}
	if info.Size() > attachmentMaxBytes {
		return appendNotice(result, fmt.Sprintf(
			"File not attached: the file is %d bytes, which exceeds the %d byte attachment limit",
			info.Size(), attachmentMaxBytes,
		))
	}

	data, err := os.ReadFile(fileAbsolutePath)
	if err != nil {
		return appendNotice(result, fmt.Sprintf("File not attached: %s", err.Error()))
	}

	name := filepath.Base(fileAbsolutePath)
	result.Content = append(result.Content,
		mcp.NewTextContent(fmt.Sprintf(
			"<h2>Attached File</h2>\n<p>File [%s] (%d bytes) is attached to this result as a base64 blob resource.</p>\n",
			name, len(data),
		)),
		mcp.NewEmbeddedResource(mcp.BlobResourceContents{
			URI:      "file://" + filepath.ToSlash(fileAbsolutePath),
			MIMEType: mimeType,
			Blob:     base64.StdEncoding.EncodeToString(data),
		}),
	)
	return result
}

func appendNotice(result *mcp.CallToolResult, text string) *mcp.CallToolResult {
	result.Content = append(result.Content, mcp.NewTextContent(fmt.Sprintf("<h2>Notice</h2>\n<p>%s</p>\n", text)))
	return result
}

# Excel MCP Server

<img src="https://github.com/negokaz/excel-mcp-server/blob/main/docs/img/icon-800.png?raw=true" width="128">

<a href="https://glama.ai/mcp/servers/@negokaz/excel-mcp-server">
  <img width="380" height="200" src="https://glama.ai/mcp/servers/@negokaz/excel-mcp-server/badge" alt="Excel Server MCP server" />
</a>

[![NPM Version](https://img.shields.io/npm/v/@negokaz/excel-mcp-server)](https://www.npmjs.com/package/@negokaz/excel-mcp-server)
[![smithery badge](https://smithery.ai/badge/@negokaz/excel-mcp-server)](https://smithery.ai/server/@negokaz/excel-mcp-server)

A Model Context Protocol (MCP) server that reads and writes MS Excel data.

## Features

- Read/Write text values
- Read/Write formulas
- Create new sheets

**🪟Windows only:**
- Live editing
- Capture screen image from a sheet

For more details, see the [tools](#tools) section.

## Requirements

- Node.js 20.x or later

## Supported file formats

- xlsx (Excel book)
- xlsm (Excel macro-enabled book)
- xltx (Excel template)
- xltm (Excel macro-enabled template)

## Installation

### Installing via NPM

excel-mcp-server is automatically installed by adding the following configuration to the MCP servers configuration.

For Windows:
```json
{
    "mcpServers": {
        "excel": {
            "command": "cmd",
            "args": ["/c", "npx", "--yes", "@negokaz/excel-mcp-server"],
            "env": {
                "EXCEL_MCP_PAGING_CELLS_LIMIT": "4000"
            }
        }
    }
}
```

For other platforms:
```json
{
    "mcpServers": {
        "excel": {
            "command": "npx",
            "args": ["--yes", "@negokaz/excel-mcp-server"],
            "env": {
                "EXCEL_MCP_PAGING_CELLS_LIMIT": "4000"
            }
        }
    }
}
```

### Installing via Docker

Build the image from the repository:

```bash
docker build -t excel-mcp-server .
```

Then add the following configuration to the MCP servers configuration.
Mount the directory containing your Excel files so the server can access them:

```json
{
    "mcpServers": {
        "excel": {
            "command": "docker",
            "args": [
                "run", "--rm", "-i",
                "-v", "/path/to/excel/files:/path/to/excel/files",
                "-e", "EXCEL_MCP_PAGING_CELLS_LIMIT",
                "excel-mcp-server"
            ],
            "env": {
                "EXCEL_MCP_PAGING_CELLS_LIMIT": "4000"
            }
        }
    }
}
```

Use the same path inside and outside the container so `fileAbsolutePath` arguments resolve.

> [!NOTE]
> The Docker image is Linux-based, so Windows-only features (live editing, screen capture) are not available.

### Installing via Smithery

To install Excel MCP Server for Claude Desktop automatically via [Smithery](https://smithery.ai/server/@negokaz/excel-mcp-server):

```bash
npx -y @smithery/cli install @negokaz/excel-mcp-server --client claude
```

<h2 id="transports">Transports</h2>

The server speaks stdio by default: the client launches the binary and talks to it over its standard input and output.

Setting `EXCEL_MCP_TRANSPORT=http` switches it to the MCP **Streamable HTTP** transport instead, so a client can connect to a long-running server over the network:

```bash
EXCEL_MCP_TRANSPORT=http EXCEL_MCP_HTTP_ADDR=localhost:8000 excel-mcp-server
```

The server then serves one endpoint (`/mcp` by default) handling `POST` for requests, `GET` for the server-to-client SSE stream, and `DELETE` to end a session. Point the client at it:

```json
{
    "mcpServers": {
        "excel": {
            "url": "http://localhost:8000/mcp"
        }
    }
}
```

> [!WARNING]
> The HTTP transport has no authentication of its own, which is why it listens on loopback by default. Put an authenticating proxy in front of it before exposing it beyond the host.

### File paths over HTTP

Over stdio the client and the server share a filesystem, so `fileAbsolutePath` means the same thing on both sides. Over HTTP it does not, so the server keeps a **workspace directory** — a temporary folder it owns (`<temp>/excel-mcp-server` unless `EXCEL_MCP_WORKSPACE_DIR` says otherwise):

- A **relative** path is resolved inside the workspace directory, and missing parent directories are created. This works on every transport, and it is the only kind of path a remote client can name safely.
- Writing to a path that does not exist yet **creates the workbook**, so a client can build one from scratch — `excel_write_to_sheet` with a relative path and `attachFile: true` writes into the workspace and hands the file back as a download, with no shared filesystem involved.
- An **absolute** path outside the workspace is refused under the HTTP transport, and accepted under stdio. `EXCEL_MCP_RESTRICT_TO_WORKSPACE` overrides that default in either direction.

The workspace is a temporary directory: treat what it holds as scratch, and take the results out as attachments.

<h2 id="tools">Tools</h2>

### `excel_describe_sheets`

List all sheet information of specified Excel file.

**Arguments:**
- `fileAbsolutePath`
    - Absolute path to the Excel file, or a path relative to the [workspace directory](#transports)

### `excel_read_sheet`

Read values from Excel sheet with pagination.

**Arguments:**
- `fileAbsolutePath`
    - Absolute path to the Excel file, or a path relative to the [workspace directory](#transports)
- `sheetName`
    - Sheet name in the Excel file
- `range`
    - Range of cells to read in the Excel sheet (e.g., "A1:C10"). [default: first paging range]
- `showFormula`
    - Show formula instead of value [default: false]
- `showStyle`
    - Show style information for cells [default: false]

### `excel_screen_capture`

**[Windows only]** Take a screenshot of the Excel sheet with pagination.

**Arguments:**
- `fileAbsolutePath`
    - Absolute path to the Excel file, or a path relative to the [workspace directory](#transports)
- `sheetName`
    - Sheet name in the Excel file
- `range`
    - Range of cells to read in the Excel sheet (e.g., "A1:C10"). [default: first paging range]

### `excel_write_to_sheet`

Write values to the Excel sheet.

**Arguments:**
- `fileAbsolutePath`
    - Absolute path to the Excel file, or a path relative to the [workspace directory](#transports)
- `sheetName`
    - Sheet name in the Excel file
- `newSheet`
    - Create a new sheet if true, otherwise write to the existing sheet
- `range`
    - Range of cells to read in the Excel sheet (e.g., "A1:C10").
- `values`
    - Values to write to the Excel sheet. If the value is a formula, it should start with "="
- `attachFile`
    - Attach the saved workbook to the result as a downloadable binary attachment [default: false]
    - The result then carries, after the usual HTML, an embedded blob resource: the whole file base64-encoded with the workbook's MIME type, so a client or an agent runtime can offer it as a download instead of a local path. Set it on the last write of a file the caller must be able to download. Files larger than 20 MB are not attached; the write still succeeds and the result says why.

### `excel_create_table`

Create a table in the Excel sheet

**Arguments:**
- `fileAbsolutePath`
    - Absolute path to the Excel file, or a path relative to the [workspace directory](#transports)
- `sheetName`
    - Sheet name where the table is created
- `range`
    - Range to be a table (e.g., "A1:C10")
- `tableName`
    - Table name to be created

### `excel_copy_sheet`

Copy existing sheet to a new sheet

**Arguments:**
- `fileAbsolutePath`
    - Absolute path to the Excel file, or a path relative to the [workspace directory](#transports)
- `srcSheetName`
    - Source sheet name in the Excel file
- `dstSheetName`
    - Sheet name to be copied

### `excel_format_range`

Format cells in the Excel sheet with style information

**Arguments:**
- `fileAbsolutePath`
    - Absolute path to the Excel file, or a path relative to the [workspace directory](#transports)
- `sheetName`
    - Sheet name in the Excel file
- `range`
    - Range of cells in the Excel sheet (e.g., "A1:C3")
- `styles`
    - 2D array of style objects for each cell. If a cell does not change style, use null. The number of items of the array must match the range size.
    - Style object properties:
        - `border`: Array of border styles (type, color, style)
        - `font`: Font styling (bold, italic, underline, size, strike, color, vertAlign)
        - `fill`: Fill/background styling (type, pattern, color, shading)
        - `numFmt`: Custom number format string
        - `decimalPlaces`: Number of decimal places (0-30)

<h2 id="configuration">Configuration</h2>

You can change the MCP Server behaviors by the following environment variables:

### `EXCEL_MCP_PAGING_CELLS_LIMIT`

The maximum number of cells to read in a single paging operation.  
[default: 4000]

### `EXCEL_MCP_TRANSPORT`

The transport to serve: `stdio` or `http` (MCP Streamable HTTP).  
[default: stdio]

### `EXCEL_MCP_HTTP_ADDR`

The address the HTTP transport listens on.  
[default: localhost:8000]

### `EXCEL_MCP_HTTP_PATH`

The endpoint path the HTTP transport serves.  
[default: /mcp]

### `EXCEL_MCP_HTTP_STATELESS`

Serve every HTTP request without a session, for deployments that cannot pin a client to one server instance.  
[default: false]

### `EXCEL_MCP_WORKSPACE_DIR`

The directory that relative `fileAbsolutePath` arguments resolve inside.  
[default: `<system temp directory>/excel-mcp-server`]

### `EXCEL_MCP_RESTRICT_TO_WORKSPACE`

Refuse any path outside the workspace directory.  
[default: true under the `http` transport, false under `stdio`]

## License

Copyright (c) 2025 Kazuki Negoro

excel-mcp-server is released under the [MIT License](LICENSE)
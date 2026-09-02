# The server is a single static Go binary, so it is built from this repository
# rather than pulled from NPM: the image then matches the checked-out source.
FROM golang:1.24-alpine AS build

ARG VERSION=docker

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/excel-mcp-server ./cmd/excel-mcp-server

FROM alpine:3.21 AS release

RUN adduser -D -u 10001 excel

# Relative file paths resolve inside the workspace directory, which is where a
# client that does not share this container's filesystem writes its workbooks.
# Mount a volume over it to keep what lands there.
ENV EXCEL_MCP_WORKSPACE_DIR=/workspace
RUN mkdir -p /workspace && chown excel:excel /workspace

# Loopback is the right default for a local process, but inside a container it
# would make the HTTP transport unreachable from the host.
ENV EXCEL_MCP_HTTP_ADDR=0.0.0.0:8000
EXPOSE 8000

COPY --from=build /out/excel-mcp-server /usr/local/bin/excel-mcp-server

USER excel
WORKDIR /workspace

ENTRYPOINT ["excel-mcp-server"]

# FML Language Server

This is a Language Server Protocol (LSP) server for the Frags DSL (FML).

## Features
- **Real-time Diagnostics**: Reports syntax errors as you type.
- **Hover Support**: Basic hover capability (placeholder).

## Building
To build the LSP server, run:
```bash
go build -o fml-lsp .
```

## Usage
The server supports multiple transport modes, which can be enabled via environment variables.

### Stdio (Default)
If no environment variables are set, the server communicates via standard input/output.
```bash
./fml-lsp
```

### TCP (Internet Service)
Set `JSON_RPC_PORT` to listen on a TCP address for remote connections.
```bash
JSON_RPC_PORT=7100 ./fml-lsp
```

### WebSocket (Web-based Editors)
Set `WEBSOCKET_PORT` to listen on a WebSocket address for browser-based editor integrations.
```bash
WEBSOCKET_PORT=7101 ./fml-lsp
```

### Simultaneous Execution
Both TCP and WebSocket modes can be enabled at the same time:
```bash
JSON_RPC_PORT=7100 WEBSOCKET_PORT=7101 ./fml-lsp
```

## Dependencies
- `github.com/owenrumney/go-lsp`: LSP 3.17 implementation.
- `github.com/theirish/fml`: The core FML parser and compiler (imported via local replace).

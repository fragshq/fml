# FML Language Server Library

This is a Language Server Protocol (LSP) library for the Frags DSL (FML). It can be integrated into other Go applications to provide LSP capabilities.

## Features
- **Real-time Diagnostics**: Reports syntax errors as you type, supporting multi-line system prompts.
- **Hover Support**: Basic hover capability.
- **Semantic Tokens**: Advanced syntax highlighting, including multi-line backtick strings.
- **Completions**: Smart completion suggestions for keywords, attributes, and types.

## Usage

As a library, you can run the LSP server in various modes:

### Stdio
Suitable for integration with most editors (VS Code, Vim, etc.).
```go
import "github.com/fragshq/fml/lsp"

// ...
err := lsp.RunStdio(context.Background())
```

### TCP
Listen on a TCP port.
```go
err := lsp.RunTCP(context.Background(), ":7100")
```

### WebSocket
Listen on a WebSocket port (e.g., for browser-based editors).
```go
err := lsp.RunWS(context.Background(), ":7101")
```

## Dependencies
- `github.com/owenrumney/go-lsp`: LSP 3.17 implementation.
- `github.com/fragshq/fml`: The core FML parser and compiler.

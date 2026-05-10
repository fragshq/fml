# FML Language Server

This is a Language Server Protocol (LSP) server for the Frags DSL (FML).

## Features
- **Real-time Diagnostics**: Reports syntax errors as you type.
- **Hover Support**: Basic hover capability (placeholder).

## Building
To build the LSP server, run:
```bash
go build -o fml-lsp main.go
```

## Usage
The server communicates via standard input/output (Stdio). It can be integrated into any editor that supports LSP (VS Code, Vim/Neovim, Emacs, etc.).

## Dependencies
- `github.com/owenrumney/go-lsp`: LSP 3.17 implementation.
- `github.com/theirish/fml`: The core FML parser and compiler (imported via local replace).

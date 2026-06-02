# FML (Frags Modeling Language)

FML is a compact, human-readable Domain Specific Language designed for defining complex LLM pipelines. It compiles to structured Frags YAML plans, enabling structured outputs, tool integration, and iterative multi-session workflows.

## Features

- **Concise Syntax**: A clean, indentation-sensitive language for LLM pipeline definition.
- **Session-Based Pipelines**: Organize your LLM tasks into logical, dependent sessions.
- **Automatic Schema Assembly**: Generates JSON Schema automatically from session definitions.
- **Tool & MCP Support**: Easily declare tool requirements (MCP, API CP, functions) with optional method allowlisting.
- **Metadata Preservation**: Comments are preserved and mapped to JSON Schema descriptions.
- **Bi-directional**: Includes a compiler (FML -> YAML) and a decompiler (YAML -> FML).
- **IDE Support**: Language Server Protocol (LSP) implementation for real-time diagnostics and highlighting.

## Installation

```bash
# Clone the repository
git clone https://github.com/theirish81/fml.git
cd fml

# Build the CLI
go build -o fml main.go
```

## Usage

The FML CLI is powered by [Cobra](https://github.com/spf13/cobra), providing a robust interface with built-in help.

### Compiling FML to YAML

```bash
./fml compile test.fml -o plan.yaml
```

### Decompiling YAML to FML

```bash
./fml decompile plan.yaml -o restored.fml
```

### Help

You can always use the `--help` flag to see available commands and options:

```bash
./fml --help
./fml compile --help
```

## Language Overview

```fml
system("You are a research assistant.")

parameter("topic", type=string)
parameter("limit", type=int, default=5)

session("research") {
    use search
    
    + Search for the latest trends in {{ .params.topic }}.
    - Summarize the top {{ .params.limit }} results.

    schema {
        trends: string[]
        summary: string
    }
}
```

For more details on the language, see [frags-dsl-spec.md](frags-dsl-spec.md).

## LSP Library

The FML LSP is provided as a library in the `./lsp` directory. It can be integrated into Go applications to provide Language Server Protocol capabilities via Stdio, TCP, or WebSocket.

Features supported:
- Syntax Diagnostics (including multi-line system prompts)
- Semantic Tokens (Highlighting including backtick raw strings)
- Autocompletion (Keywords, Types, Attributes)
- Basic Hover

For integration details, see [lsp/README.md](lsp/README.md).

## Project Structure

- `parser/`: Lexer and AST definition.
- `compiler/`: Logic for DSL to YAML transformation.
- `decompiler/`: Logic for YAML to DSL transformation.
- `lsp/`: Language Server Protocol library.
- `main.go`: CLI entry point.

## License

MIT

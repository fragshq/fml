# Frags DSL Compiler - Project Overview & Guidelines

This document serves as a foundational reference for the Frags DSL compiler, capturing its architecture, design decisions, and current implementation state.

## 1. Project Overview
The Frags DSL compiler is a Go-based tool that transforms a compact, human-readable Domain Specific Language (DSL) into structured Frags YAML plans. It is designed to facilitate the definition of complex LLM pipelines with structured outputs, tools, and iterative sessions.

### Key Components:
- **Lexer (`parser/lexer.go`)**: A manual, stateful lexer that handles indentation-sensitive prompt blocks, balanced-parentheses code/expression segments, and context-aware attribute capture.
- **Parser (`parser/ast.go`)**: Uses `participle/v2` to build an Abstract Syntax Tree (AST) that mirrors the DSL's grammar.
- **Compiler (`compiler/compiler.go`)**: Implements the multi-pass transformation logic, including session merging, root schema assembly, and "off-side rule" indentation stripping.
- **Decompiler (`decompiler/decompiler.go`)**: Performs the inverse operation, converting a validated Frags YAML plan back into idiomatic FML source code, preserving comments and structure where possible.
- **LSP Server (`lsp/main.go`)**: A Language Server Protocol implementation providing real-time diagnostics, semantic highlighting, and basic hover support for modern IDE integration.
- **CLI (`main.go`)**: A robust command-line interface powered by Cobra for compiling and decompiling FML/YAML files.

## 2. Technical Architecture

### Stateful Lexing
The lexer uses a state-machine approach to handle complex DSL constructs:
- **Prompt Detection**: Identifies `+ ` (pre-prompt) and `- ` (prompt) markers at the start of lines and captures multi-line blocks based on indentation levels.
- **Balanced Capture**: Used for `code(...)` and `$(...)` blocks to ensure nested parentheses are correctly tokenized as a single unit.
- **Attribute Context**: Switches to greedy consumption for session attributes like `expect=` and `iterate=` to capture raw expressions without premature termination at standard punctuation.

### Schema Assembly Logic
The compiler implements a unique grouping strategy for the output JSON Schema:
- **Session Grouping**: Each session (e.g., `session("gather")`) contributes a single top-level property to the root schema, named after the session itself (or overridden via `target=`).
- **Iteration Handling**: If a session has an `iterate` attribute, its corresponding root property is automatically wrapped in a JSON Schema `array` type.
- **Flattening & Merging**: Multiple `schema` blocks within a session are merged into the session's object-type property. Anonymous schemas (`schema T[]`) directly set the session's property type.

### Comment & Metadata Preservation
- **Preservation Rule**: Comments adjacent to fields are prioritized as JSON Schema `description` fields.
- **YAML Metadata**: Orphaned or block-level comments are captured and emitted as native YAML `HeadComment` or `LineComment` metadata using `yaml.Node`.

## 3. Development Guidelines

### Language Grammar
- Always refer to `frags-dsl-spec.md` for the authoritative PEG grammar and compilation rules.
- **Key Features**:
    - `system("...")`: Sets the global system prompt.
    - `parameter("name", type=..., default=..., title=...)`: Defines an input parameter.
    - `transformer("name") { ... }`: Defines reusable output transformers.
    - `call("tool") [-> [ns:]var] [{ ... }]`: Tool invocations with optional namespaced argument mapping and optional body.
    - `session("name", after="prev", expect=...) { ... }`: Sequential pipeline steps.
    - `use mcp|apicp|search ...`: Declares tool requirements.
    - Enums (`"a"|"b"`) and optional fields (`field?: type`) are natively supported.

### Testing Standard
- **Rigor**: All new features or bug fixes MUST include corresponding test cases in `parser/lexer_test.go`, `parser/parser_test.go`, `compiler/compiler_test.go`, or `decompiler/decompiler_test.go`.
- **Validation**: Ensure that all existing test scenarios pass before committing changes.

## 4. Current State & Roadmap
- [x] Manual stateful lexer with robust error handling.
- [x] Full AST coverage for the Frags DSL specification.
- [x] Accurate session-grouped schema assembly with automatic array wrapping.
- [x] Precise YAML comment placement and description mapping.
- [x] Decompiler for converting YAML plans back to FML.
- [x] LSP Server with diagnostics, semantic highlighting, and autocompletion.
- [x] High-coverage unit test suite.

**Future Considerations**:
- [ ] Enhanced semantic validation for circular dependencies and variable scope.
- [ ] Integration with external tool definitions (MCP, API CP) for better reference checking.
- [ ] Advanced LSP features: Goto Definition and Rename refactoring.
- [ ] Formatter (`fml fmt`) to standardize code style across projects.

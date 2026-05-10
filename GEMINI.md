# Frags DSL Compiler - Project Overview & Guidelines

This document serves as a foundational reference for the Frags DSL compiler, capturing its architecture, design decisions, and current implementation state.

## 1. Project Overview
The Frags DSL compiler is a Go-based tool that transforms a compact, human-readable Domain Specific Language (DSL) into structured Frags YAML plans. It is designed to facilitate the definition of complex LLM pipelines with structured outputs, tools, and iterative sessions.

### Key Components:
- **Lexer (`parser/lexer.go`)**: A manual, stateful lexer that handles indentation-sensitive prompt blocks, balanced-parentheses code/expression segments, and context-aware attribute capture.
- **Parser (`parser/ast.go`)**: Uses `participle/v2` to build an Abstract Syntax Tree (AST) that mirrors the DSL's grammar.
- **Compiler (`compiler/compiler.go`)**: Implements the multi-pass transformation logic, including session merging, root schema assembly, and "off-side rule" indentation stripping.
- **CLI (`main.go`)**: Provides a simple command-line interface for compiling `.frags` files.

## 2. Technical Architecture

### Stateful Lexing
The lexer uses a state-machine approach to handle complex DSL constructs:
- **Prompt Detection**: Identifies `- ` markers at the start of lines and captures multi-line blocks based on indentation.
- **Balanced Capture**: Used for `code(...)` and `$(...)` blocks to ensure nested parentheses are correctly tokenized as a single unit.
- **Attribute Context**: Switches to greedy consumption for session attributes like `expect=` and `iterate=` to capture raw expressions without premature termination at standard punctuation.

### Schema Assembly Logic
The compiler implements a unique grouping strategy for the output JSON Schema:
- **Session Grouping**: Each session (e.g., `session("gather")`) contributes a single top-level property to the root schema, named after the session itself.
- **Iteration Handling**: If a session has an `iterate` attribute, its corresponding root property is automatically wrapped in a JSON Schema `array` type.
- **Flattening & Merging**: Multiple `schema` blocks within a session are merged into the session's object-type property. Anonymous schemas (`schema [T]`) directly set the session's property type.

### Comment & Metadata Preservation
- **Preservation Rule**: Comments adjacent to fields are prioritized as JSON Schema `description` fields.
- **YAML Metadata**: Orphaned or block-level comments are captured and emitted as native YAML `HeadComment` or `LineComment` metadata using `yaml.Node`.

## 3. Development Guidelines

### Language Grammar
- Always refer to `frags-dsl-spec.md` for the authoritative PEG grammar and compilation rules.
- Changes to the lexer or parser must be synchronized with updates to the specification.

### Testing Standard
- **Rigor**: All new features or bug fixes MUST include corresponding test cases in `parser/lexer_test.go`, `parser/parser_test.go`, or `compiler/compiler_test.go`.
- **Validation**: Ensure that all 19 existing test scenarios pass before committing changes.

### Adding New Features
1. Update `frags-dsl-spec.md` with the proposed syntax and rules.
2. Implement lexical support in `parser/lexer.go` (if new tokens or states are needed).
3. Update AST nodes in `parser/ast.go`.
4. Implement compilation logic in `compiler/compiler.go`.
5. Add unit and end-to-end tests to verify the feature.

## 4. Current State & Roadmap
- [x] Manual stateful lexer with robust error handling.
- [x] Full AST coverage for the Frags DSL specification.
- [x] Accurate session-grouped schema assembly with automatic array wrapping.
- [x] Support for complex objects and raw expressions in variables/args.
- [x] Precise YAML comment placement and description mapping.
- [x] High-coverage unit test suite (19 tests).

**Future Considerations**:
- Enhanced semantic validation for circular dependencies and variable scope.
- Integration with external tool definitions (MCP, API CP) for better reference checking.
- Language Server Protocol (LSP) support for enhanced developer experience.

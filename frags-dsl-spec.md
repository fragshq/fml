# Frags DSL — Language Specification & Compiler Reference

> This document defines the syntax and compilation rules for the Frags DSL — a compact
> language that compiles to Frags YAML plan files. It is intended to be a complete and
> unambiguous reference for implementing a compiler.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Lexical Rules](#2-lexical-rules)
3. [Grammar](#3-grammar)
4. [Top-Level Blocks](#4-top-level-blocks)
   - 4.1 `system`
   - 4.2 `parameters`
   - 4.3 `transformer`
   - 4.4 `call` (plan-level)
   - 4.5 `components`
5. [Session Block](#5-session-block)
   - 5.1 Header & Attributes
   - 5.2 `set`
   - 5.3 `use`
   - 5.4 `call` (session-level)
   - 5.5 `context`
   - 5.6 Prompt Lines
   - 5.7 `schema`
6. [Type System](#6-type-system)
7. [Comment & Description Resolution](#7-comment--description-resolution)
8. [Compiler Output Rules](#8-compiler-output-rules)
   - 8.1 Plan-level fields
   - 8.2 `parameters`
   - 8.3 `requiredTools` inference
   - 8.4 Sessions
   - 8.5 `preCalls`
   - 8.6 `transformers`
   - 8.7 Root schema assembly
   - 8.8 `components`
9. [Full Example](#9-full-example)

---

## 1. Overview

A Frags DSL file is a plain-text file (recommended extension: `.frags`) that describes a
multi-session LLM pipeline. The compiler translates it into a single Frags-compatible YAML file.

The top level of a DSL file is a flat sequence of **blocks** and **statements**. Order matters
for readability but the compiler does not require a specific ordering of top-level blocks, with
one exception: `components` must be resolved before `$ref` usages are validated.

---

## 2. Lexical Rules

```
NEWLINE      = "\n" | "\r\n"
SP           = " " | "\t"                     # one or more horizontal spaces
INDENT(n)    = exactly n spaces of leading whitespace (tabs count as 1 space)
BLANK_LINE   = a line containing only SP* followed by NEWLINE
IDENTIFIER   = [a-zA-Z_][a-zA-Z0-9_]*
STRING_LIT   = '"' ( [^"\\] | '\\' . )* '"'   # standard JSON-style escaping
NUMBER_LIT   = '-'? [0-9]+ ( '.' [0-9]+ )?
BOOL_LIT     = "true" | "false"
COMMENT      = '#' [^\n]* NEWLINE              # leading comment (whole line)
INLINE_CMT   = '#' [^\n]*                      # inline comment (end of line)
RAW_JS       = balanced parentheses content    # see §3 CodeBlock
RAW_EXPR     = balanced parentheses content    # Golang Expr syntax
```

String literals support standard escapes: `\"`, `\\`, `\n`, `\t`.

**Go template strings** (`{{ .params.x }}`, `{{ .vars.y }}`, `{{ .it.field }}`,
`{{ .context.field }}`) appear inside `STRING_LIT` values and prompt lines and are passed
through verbatim to the YAML output without interpretation by the compiler.

**Expr values** are delimited by `$(` … `)`. The content is a Golang Expr expression and is
passed through verbatim.

---

## 3. Grammar

The grammar is presented in PEG notation. Whitespace (spaces, tabs, blank lines) between
tokens is ignored unless explicitly significant (indentation in prompt lines and continuation
lines).

```peg
# ── Top level ─────────────────────────────────────────────────────────────────

Plan         ← Statement* EOF

Statement    ← COMMENT
             / SystemBlock
             / ParametersBlock
             / ComponentsBlock
             / SessionBlock
             / CallBlock
             / TransformerBlock
             / SetStmt

# ── system ────────────────────────────────────────────────────────────────────

SystemBlock  ← "system" "(" STRING_LIT ")"

# ── parameters ────────────────────────────────────────────────────────────────

ParametersBlock ← "parameters" "{" ParamEntry* "}"

ParamEntry   ← LeadingComment* IDENTIFIER ":" TypeExpr DefaultVal? INLINE_CMT? NEWLINE

DefaultVal   ← "=" LiteralValue

# ── transformer ───────────────────────────────────────────────────────────────

TransformerBlock ← "transformer" "(" STRING_LIT ")" "{" TransformerField* "}"

TransformerField ← TransformerTrigger "=" STRING_LIT NEWLINE
                 / "jmesPath"         "=" STRING_LIT NEWLINE

TransformerTrigger ← "on_function" | "on_input" | "on_resource"

# ── call (plan-level and session-level share the same syntax) ─────────────────

CallBlock    ← "call" "(" STRING_LIT ")" ("->" IDENTIFIER)? "{" CallField* "}"

CallField    ← IDENTIFIER "=" Value NEWLINE
             / CodeBlock

CodeBlock    ← "code" "(" RawJS ")"

RawJS        ← ( [^()] / "(" RawJS ")" )*    # balanced parens, recursive

# ── set ───────────────────────────────────────────────────────────────────────

SetStmt      ← "set" IDENTIFIER "=" Value NEWLINE

# ── components ────────────────────────────────────────────────────────────────

ComponentsBlock ← "components" "{" ComponentItem* "}"

ComponentItem   ← SchemaComponent
                / PromptComponent

SchemaComponent ← "schema" "(" STRING_LIT ")" "{" SchemaField* "}"
PromptComponent ← "prompt" "(" STRING_LIT ")" "{" STRING_LIT "}"

# ── session ───────────────────────────────────────────────────────────────────

SessionBlock ← "session" "(" STRING_LIT SessionAttrList? ")" "{" SessionStmt* "}"

SessionAttrList ← ("," SessionAttr)+

SessionAttr  ← "after"   "=" STRING_LIT      # dependsOn[].session
             / "expect"  "=" RawExpr          # dependsOn[].expression (unquoted Expr)
             / "iterate" "=" RawExpr          # iterateOn (unquoted Expr)

SessionStmt  ← COMMENT
             / SetStmt
             / UseStmt
             / CallBlock
             / ContextStmt
             / PromptLines
             / SchemaBlock

UseStmt      ← "use" ToolType IDENTIFIER? NEWLINE

ToolType     ← "mcp" | "apicp" | "collection" | "function" | "search"

ContextStmt  ← "context" (BOOL_LIT / STRING_LIT) NEWLINE

# ── prompt lines ──────────────────────────────────────────────────────────────

PromptLines  ← PromptItem+

PromptItem   ← DashLine ContinuationLine*

DashLine     ← SP* "-" SP InlineText NEWLINE

# A continuation line must be indented strictly deeper than the column of
# the "-" that opened the item. A BLANK_LINE terminates the item without
# being consumed. A line at <= the "-" column terminates the item.
ContinuationLine ← !BLANK_LINE &DeepIndent InlineText NEWLINE

DeepIndent   ← SP{n+1,}    # n = column index (0-based) of the opening "-"

InlineText   ← [^\n]+

# ── schema (inside session or components) ─────────────────────────────────────

SchemaBlock  ← "schema" "{" SchemaField* "}"
             / "schema" TypeExpr

SchemaField  ← LeadingComment* IDENTIFIER "?"? ":" TypeExpr InlineSchemaExt? INLINE_CMT? NEWLINE
             / LeadingComment* IDENTIFIER "?"? ":" ObjectBody                 INLINE_CMT? NEWLINE

InlineSchemaExt ← "{" SchemaField* "}"   # for inline object expansion after ":"

# ── types ─────────────────────────────────────────────────────────────────────

TypeExpr     ← ArrayTypeSuffix
             / ArrayTypeBracket
             / ObjectBody
             / RefType
             / EnumType
             / ScalarType

ScalarType   ← "string" | "int" | "float" | "bool" | "any"

ArrayTypeSuffix  ← ScalarType "[]"                   # string[], int[]
ArrayTypeBracket ← "[" TypeExpr "]"                  # [string], [{...}], [$ref]

ObjectBody   ← "{" SchemaField* "}"                  # inline object

RefType      ← "$" IDENTIFIER                        # $ref to component schema

EnumType     ← EnumValue ("|" EnumValue)+
EnumValue    ← STRING_LIT / IDENTIFIER               # "active"|"inactive" or go|no_go

# ── shared primitives ─────────────────────────────────────────────────────────

LeadingComment ← SP* "#" InlineText NEWLINE          # comment line above a field

Value        ← LiteralValue
             / ExprValue
             / ObjectValue

LiteralValue ← STRING_LIT | NUMBER_LIT | BOOL_LIT

ExprValue    ← "$(" RawExpr ")"
RawExpr      ← ( [^()] / "(" RawExpr ")" )*  # balanced parens, recursive

ObjectValue  ← "{" (ObjectEntry (","? ObjectEntry)*)? "}"
ObjectEntry  ← IDENTIFIER ":" Value
```

---

## 4. Top-Level Blocks

### 4.1 `system`

```
system("You are a precise, structured content generator.")
```

Compiles to the plan-level `systemPrompt` field.

```yaml
systemPrompt: "You are a precise, structured content generator."
```

---

### 4.2 `parameters`

Each entry declares a named input parameter with a type and optional default value.

```
parameters {
    topic:      string
    max_items:  int = 10
    # The user's full name
    full_name:  string = "Anonymous"
    tags:       [string]
    address: {
        street: string
        city:   string
    }
}
```

**Compilation rules:**

- Each entry becomes one element of the `parameters` array.
- The `name` field is the identifier.
- The `schema` field is the compiled JSON Schema for the type (see §6).
- If a `default` is present, emit `default: <value>`.
- The `?` optional marker is **not** valid on parameters (parameters are either required
  or have a default; omitting `?` and omitting a default makes the parameter required).
- A leading comment on the line immediately above the entry becomes the parameter's
  `description` field inside `schema`.
- An inline comment on the same line as the entry also becomes `description`; if both
  leading and inline are present, the inline comment wins.

```yaml
parameters:
  - name: topic
    schema:
      type: string
  - name: max_items
    schema:
      type: integer
    default: 10
  - name: full_name
    schema:
      type: string
      description: "The user's full name"
    default: "Anonymous"
  - name: tags
    schema:
      type: array
      items:
        type: string
  - name: address
    schema:
      type: object
      properties:
        street: { type: string }
        city:   { type: string }
      required: [street, city]
```

---

### 4.3 `transformer`

```
transformer("slimRepos") {
    on_function = "listRepositories"
    jmesPath    = "[*].{name: name, url: html_url}"
}
```

**Trigger field mapping:**

| DSL field | YAML field |
|-----------|-----------|
| `on_function` | `onFunctionOutput` |
| `on_input` | `onFunctionInput` |
| `on_resource` | `onResource` |

Exactly one trigger field must be present. `jmesPath` is always required.

```yaml
transformers:
  - name: slimRepos
    onFunctionOutput: listRepositories
    jmesPath: "[*].{name: name, url: html_url}"
```

---

### 4.4 `call` (plan-level)

Plan-level `call` blocks compile to the plan-level `preCalls` array. See §8.5 for the
full compilation rules shared with session-level calls.

```
call("splitTags") -> tagList {
    raw = "{{ .context.tagString }}"
    code(
        args.raw.split(',').map(t => t.trim())
    )
}
```

---

### 4.5 `components`

```
components {
    schema("Address") {
        street: string
        city:   string
    }
    prompt("systemBase") {
        "You are a precise assistant."
    }
}
```

**Compilation rules:**

- `schema(name)` blocks compile to entries under `components.schemas`.
- `prompt(name)` blocks compile to entries under `components.prompts`.
- Schema fields follow the same type and `required` rules as session schemas (see §6).
- Component schemas can be referenced anywhere a type is expected using `$name`.

```yaml
components:
  schemas:
    Address:
      type: object
      properties:
        street: { type: string }
        city:   { type: string }
      required: [street, city]
  prompts:
    systemBase: "You are a precise assistant."
```

---

## 5. Session Block

```
session("mySession", after="prev", expect=context.flag==true, iterate=context.items) {
    ...
}
```

### 5.1 Header & Attributes

| DSL attribute | YAML output | Notes |
|---------------|-------------|-------|
| `after="x"` | `dependsOn: [{session: x}]` | ordering + success gate |
| `expect=expr` | `dependsOn: [{expression: "expr"}]` | conditional gate |
| `after="x", expect=expr` | `dependsOn: [{session: x, expression: "expr"}]` | both in same entry |
| `iterate=expr` | `iterateOn: "expr"` | session's schema slice must be array type |

`after` and `expect` may appear multiple times to produce multiple `dependsOn` entries:

```
session("s", after="a", expect=context.x>0, after="b")
```

```yaml
dependsOn:
  - session: a
    expression: "context.x>0"
  - session: b
```

When `after` and `expect` appear adjacently (no other attribute between them), they are
merged into the same `dependsOn` entry. When they appear non-adjacently or when a second
`after` appears, a new entry is started. The compiler processes attributes left to right.

---

### 5.2 `set`

Declares session-level variables. Compiles to the session's `vars` map.

```
set myVar = "hello"
set count = $(params.items | length(@))
set config = {
    limit: 10,
    debug: true
}
```

```yaml
vars:
  myVar: "hello"
  count: "$(params.items | length(@))"
  config:
    limit: 10
    debug: true
```

At plan level (outside any session), `set` compiles to the plan-level `vars` map.

When the value is an `ExprValue` (`$(…)`), emit the expression string as-is (including the
`$(` `)` delimiters) into the YAML string value.

---

### 5.3 `use`

Declares a tool available to the LLM in this session.

```
use mcp salesforce
use search
use apicp my_api
use collection http
use function custom_fn
```

**Compilation rules:**

- Each `use` statement adds an entry to the session's `tools` array.
- For every `use` except `use search`, the compiler also adds the tool to the plan-level
  `requiredTools` array (deduplicated by `type` + `name`).
- `use search` is **never** added to `requiredTools`.

| DSL | Session `tools` entry | Added to `requiredTools`? |
|-----|-----------------------|--------------------------|
| `use mcp name` | `{type: mcp, name: name}` | Yes |
| `use apicp name` | `{type: apicp, name: name}` | Yes |
| `use collection name` | `{type: collection, name: name}` | Yes |
| `use function name` | `{type: function, name: name}` | Yes |
| `use search` | `{type: internet_search}` | No |

---

### 5.4 `call` (session-level)

Session-level `call` blocks compile to the session's `preCalls` array.
See §8.5 for the full compilation rules.

---

### 5.5 `context`

```
context true
context "Categories found: {{ .context.categories }}"
```

Compiles directly to the session's `context` field:

```yaml
context: true
# or
context: "Categories found: {{ .context.categories }}"
```

---

### 5.6 Prompt Lines

Prompt lines are the only session statements that are order-sensitive relative to each other.
They are introduced by `-` at the current indentation level.

**Syntax:**

```
- First line of a prompt item
  this line continues the item because it is indented deeper
  so does this one

- This blank line above ended the previous item. This is item 2.
- This is item 3 (the prompt).
```

**Termination rules for a single item:**

An item started by a `-` at column `c` (0-based) accumulates continuation lines as long as:
1. The next line is not blank, **and**
2. The next line's leading whitespace is strictly greater than `c` spaces.

A blank line or a line with ≤ `c` leading spaces terminates the item (without consuming
the terminating line).

**Content assembly:**

1. Take the text after `- ` on the dash line (trimmed of leading/trailing whitespace).
2. For each continuation line, strip the common leading indent (the `c+1` spaces that mark
   it as a continuation) and trim trailing whitespace.
3. Join all lines with `\n`.

**Compilation rules:**

| Number of items | Output |
|-----------------|--------|
| 0 | Neither `prompt` nor `prePrompt` is emitted. |
| 1 | `prompt: "<item>"` |
| 2 | `prePrompt: "<item1>"` (string, not array), `prompt: "<item2>"` |
| 3+ | `prePrompt: ["<item1>", "<item2>", ...]`, `prompt: "<lastItem>"` |

The last item is always the `prompt`. All preceding items form `prePrompt`.
When there are exactly 2 items, `prePrompt` is emitted as a plain string, not a
single-element array.

---

### 5.7 `schema`

A `schema` block inside a session declares the portion of the root output object that this
session is responsible for filling.

**Syntax variants:**

- `schema { ...fields... }`: Defines an object containing named fields.
- `schema TypeExpr`: Defines a direct type (anonymous schema), useful for sessions producing
  a single scalar or array.

**Compilation rules:**

- Each session contributes exactly one property to the **root** schema object.
- This property is named after the session (e.g., `session("mySess")` → property `mySess`).
- The property is annotated with `x-session: <sessionName>`.
- The property is added to the root schema's `required` array unless the schema block
  is anonymous and marked optional (`schema Type?`).
- If `iterate` is set on the session, the property's type is automatically wrapped in an
  `array` if it isn't already one.
- Multiple `schema` blocks within the same session are merged into the same session-named
  property (only valid for the `{}` syntax).

Full type compilation rules are in §6.

---

## 6. Type System

The DSL type syntax unifies parameter types and schema field types. Both notations are
supported:

| Notation | Meaning |
|----------|---------|
| `string` | scalar string |
| `int` | integer |
| `float` | number (floating point) |
| `bool` | boolean |
| `any` | empty schema `{}` |
| `T[]` | array of T (suffix notation) |
| `[T]` | array of T (bracket notation) |
| `[{...}]` | array of inline object |
| `{...}` | inline object |
| `$Name` | `$ref: '#/components/schemas/Name'` |
| `a\|b\|c` | enum with values a, b, c |
| `T?` | T, but field is excluded from parent `required` |

**JSON Schema mapping:**

| DSL type | JSON Schema output |
|----------|--------------------|
| `string` | `{type: string}` |
| `int` | `{type: integer}` |
| `float` | `{type: number}` |
| `bool` | `{type: boolean}` |
| `any` | `{}` |
| `string[]` or `[string]` | `{type: array, items: {type: string}}` |
| `[{f: string}]` | `{type: array, items: {type: object, properties: {f: {type: string}}, required: [f]}}` |
| `{f: string}` | `{type: object, properties: {f: {type: string}}, required: [f]}` |
| `go\|no_go` | `{enum: [go, no_go]}` |
| `"a"\|"b"` | `{enum: [a, b]}` |
| `$Name` | `{$ref: '#/components/schemas/Name'}` |

**`required` array rules:**

- A `type: object` node has a `required` array containing all child field names that do
  **not** carry `?`.
- If all fields are optional (all have `?`), emit `required: []` or omit the field entirely
  (prefer omitting).
- `type: array` nodes never have a `required` field directly on them.
- `$ref` nodes are emitted as-is; no `required` is added at the reference site.

---

## 7. Comment & Description Resolution

Comments serve two purposes depending on context:

**As YAML comments** — when not adjacent to a field with a `description` slot, comments are
emitted as YAML `# …` comments in the output.

**As descriptions** — when adjacent to a schema field, parameter entry, or component schema
field, the comment becomes the `description` property of that field's JSON Schema node.

**Adjacency rules:**

| Comment position | Rule |
|-----------------|------|
| Inline `# text` at the end of a field line | Becomes `description`. Wins over leading comment. |
| Leading `# text` on the line immediately above a field | Becomes `description` if no inline comment is present on the field line. Multiple consecutive leading comment lines are joined with a space. |
| Comment not directly above or on a field | Emitted as a YAML `# …` comment. |

**Examples:**

```
# This becomes a YAML comment (not adjacent to any field)

parameters {
    # The user's given name
    first_name: string          # inline wins → description: "inline wins"

    # This is the description
    last_name: string

    age: int  # age in years   → description: "age in years"
}
```

Leading comment lines are stripped of the `# ` prefix and trimmed before being used as
descriptions.

---

## 8. Compiler Output Rules

The compiler produces a single YAML document. This section specifies how each DSL construct
maps to YAML fields and the order in which the output is assembled.

### 8.1 Plan-level fields

The top-level YAML object has the following fields, emitted in this order when present:

```yaml
systemPrompt: ...
parameters: [...]
vars: {...}
requiredTools: [...]
transformers: [...]
preCalls: [...]
sessions: {...}
schema: {...}
components: {...}
```

Fields with no content are omitted entirely.

---

### 8.2 `parameters`

See §4.2. Each parameter entry is emitted in declaration order.

---

### 8.3 `requiredTools` inference

The compiler collects all `use` statements across all sessions and builds `requiredTools`
automatically. Rules:

- Deduplicate by `(type, name)` pair.
- `use search` is never included.
- Order: first occurrence order across sessions (top-to-bottom, in the order sessions appear).

```yaml
requiredTools:
  - name: salesforce
    type: mcp
  - name: my_api
    type: apicp
```

---

### 8.4 Sessions

Sessions are emitted under the `sessions` map, keyed by the session name string.

**Field emission order within a session:**

```yaml
sessions:
  mySession:
    dependsOn: [...]
    iterateOn: "..."
    vars: {...}
    tools: [...]
    preCalls: [...]
    context: ...
    prePrompt: ...    # or array
    prompt: "..."
```

Fields with no content are omitted.

**`dependsOn` assembly** (see §5.1): entries are built left-to-right from the session
header attributes.

---

### 8.5 `preCalls`

Applies equally to plan-level and session-level `call` blocks.

```
call("label") -> myVar {
    param1 = "value"
    param2 = 42
    code(
        args.param1.split(',').map(s => s.trim())
    )
}
```

```yaml
- name: label
  args:
    param1: "value"
    param2: 42
  code: "args.param1.split(',').map(s => s.trim())"
  in: vars
  var: myVar
```

**Rules:**

| DSL construct | YAML field | Notes |
|---------------|-----------|-------|
| `call("name")` | `name: name` | Always present |
| `-> varName` | `in: vars`, `var: varName` | When binding present |
| no `->` | `in: ai` | No `var` field emitted |
| key-value pairs | `args: {key: value}` | Omit `args` if empty. Support nested objects and `$(...)`. |
| `code(...)` | `code: "..."` | JS expression; strip outer whitespace |

When `code` is present, `name` acts as a label (not a tool reference). The `args` map
provides inputs accessible as `args.key` inside the code expression.

The code content is the raw JS expression from inside `code(...)`, with leading and trailing
whitespace trimmed. The parentheses of `code(...)` are not included in the output value.

---

### 8.6 `transformers`

See §4.3. Transformers are emitted in declaration order under the plan-level `transformers`
array.

---

### 8.7 Root schema assembly

The compiler collects all `schema` blocks from all sessions and assembles them into the
plan-level `schema` object.

**Rules:**

1. The root schema is always `type: object`.
2. Each session declared in the file contributes exactly one property to the root object.
3. The property name matches the **session name**.
4. The property value is the compiled JSON Schema for that session (either an object
   merging all `SchemaField` entries or a single `TypeExpr`).
5. Each such property is annotated with `x-session: <sessionName>` at the property level.
6. The root `required` array contains all session property names (unless a session schema
   was anonymous and marked optional).
7. The order of properties in the root schema follows the order sessions first appear in the file.
8. If two different sessions would result in the same property name (impossible since session
   names are unique), it is a compile error.

**Example:**

Session `gather` declares:
```
schema {
    overview: {
        summary:    string
        keyPoints:  [string]
    }
}
```

Session `elaborate` declares:
```
schema {
    details?: {
        body: string
    }
}
```

Root schema output:
```yaml
schema:
  type: object
  properties:
    overview:
      x-session: gather
      type: object
      properties:
        summary:
          type: string
        keyPoints:
          type: array
          items:
            type: string
      required: [summary, keyPoints]
    details:
      x-session: elaborate
      type: object
      properties:
        body:
          type: string
      required: [body]
  required: [overview]
```

---

### 8.8 `components`

See §4.5. Emitted as:

```yaml
components:
  schemas:
    Name:
      type: object
      properties: ...
      required: [...]
  prompts:
    name: "..."
```

`components.schemas` is omitted if no `schema(...)` components are defined.
`components.prompts` is omitted if no `prompt(...)` components are defined.

---

## 9. Full Example

### DSL input

```
system("You are a precise research assistant.")

parameters {
    topic:    string
    # Maximum number of results to return
    max_results: int = 5
}

set defaultRegion = "eu-west"

transformer("slimResults") {
    on_function = "searchDocuments"
    jmesPath    = "[*].{id: id, title: title, snippet: snippet}"
}

session("gather") {
    use search
    use mcp knowledge_base

    call("searchDocuments") -> rawDocs {
        query   = "{{ .params.topic }}"
        limit   = 10
    }

    - Search for information about {{ .params.topic }}.
      Focus on the most recent and relevant sources.
      Use both the web and the knowledge base.
    - Using the search results above, extract the overview and key points
      required by the schema.
}

session("elaborate", after="gather", expect=context.overview.keyPoints != null) {
    context "Overview: {{ .context.overview }}"

    - Expand each key point into a detailed explanation.
      Be concise but thorough. Limit to {{ .params.max_results }} points.
}

components {
    schema("SourceRef") {
        id:    string
        title: string
        url:   string?
    }
}

# Sessions define the schema slices they own

session("gather") {
    schema {
        overview: {
            summary:   string
            keyPoints: [string]
            sources:   [$SourceRef]
        }
    }
}

session("elaborate") {
    schema {
        details: {
            # Expanded explanation per key point
            explanations: [string]
            confidence:   float?
        }
    }
}
```

> **Note:** In a real plan, each session is declared once; schema blocks are included inside
> their session. They are shown separately above only for illustration. The compiler merges
> multiple blocks for the same session name (schema block may appear anywhere inside the
> session body).

### YAML output

```yaml
systemPrompt: "You are a precise research assistant."

parameters:
  - name: topic
    schema:
      type: string
  - name: max_results
    schema:
      type: integer
      description: "Maximum number of results to return"
    default: 5

vars:
  defaultRegion: "eu-west"

requiredTools:
  - name: knowledge_base
    type: mcp

transformers:
  - name: slimResults
    onFunctionOutput: searchDocuments
    jmesPath: "[*].{id: id, title: title, snippet: snippet}"

sessions:
  gather:
    tools:
      - type: internet_search
      - type: mcp
        name: knowledge_base
    preCalls:
      - name: searchDocuments
        args:
          query: "{{ .params.topic }}"
          limit: 10
        in: vars
        var: rawDocs
    prePrompt: >-
      Search for information about {{ .params.topic }}.
      Focus on the most recent and relevant sources.
      Use both the web and the knowledge base.
    prompt: >-
      Using the search results above, extract the overview and key points
      required by the schema.

  elaborate:
    dependsOn:
      - session: gather
        expression: "context.overview.keyPoints != null"
    context: "Overview: {{ .context.overview }}"
    prompt: >-
      Expand each key point into a detailed explanation.
      Be concise but thorough. Limit to {{ .params.max_results }} points.

schema:
  type: object
  properties:
    gather:
      x-session: gather
      type: object
      properties:
        overview:
          type: object
          properties:
            summary:
              type: string
            keyPoints:
              type: array
              items:
                type: string
            sources:
              type: array
              items:
                $ref: '#/components/schemas/SourceRef'
          required: [summary, keyPoints, sources]
      required: [overview]
    elaborate:
      x-session: elaborate
      type: array
      items:
        type: object
        properties:
          explanations:
            type: array
            items:
              type: string
          confidence:
            type: number
        required: [explanations]
  required: [gather, elaborate]

components:
  schemas:
    SourceRef:
      type: object
      properties:
        id:
          type: string
        title:
          type: string
        url:
          type: string
      required: [id, title]
```

package lsp

import (
	"context"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
)

func (h *FMLHandler) Hover(ctx context.Context, params *lsp.HoverParams) (*lsp.Hover, error) {
	text, ok := h.documents[params.TextDocument.URI]
	if !ok {
		return nil, nil
	}

	lines := strings.Split(text, "\n")
	if params.Position.Line >= len(lines) {
		return nil, nil
	}

	line := lines[params.Position.Line]
	pos := params.Position.Character
	if pos >= len(line) {
		pos = len(line) - 1
	}
	if pos < 0 {
		return nil, nil
	}

	// Extract the word under the cursor
	start := pos
	for start > 0 && isWordChar(line[start-1]) {
		start--
	}
	end := pos
	for end < len(line) && isWordChar(line[end]) {
		end++
	}

	// Also check if we have a trailing '?' for schema?
	word := line[start:end]
	if end < len(line) && line[end] == '?' && word == "schema" {
		word = "schema?"
	}

	doc, ok := languageDocs[word]
	if !ok {
		return nil, nil
	}

	return &lsp.Hover{
		Contents: lsp.MarkupContent{
			Kind:  lsp.Markdown,
			Value: doc,
		},
		Range: &lsp.Range{
			Start: lsp.Position{Line: params.Position.Line, Character: start},
			End:   lsp.Position{Line: params.Position.Line, Character: end},
		},
	}, nil
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '+' || c == '-'
}

var languageDocs = map[string]string{
	"system":      "**system(\"prompt\")**\n\nSets the global system prompt for the entire plan. This prompt provides high-level instructions to the LLM across all sessions.",
	"parameter":   "**parameter(\"name\", ...)**\n\nDefines a named input parameter for the plan. Parameters can have a `type`, a `default` value, a `title`, and an `enum`.",
	"type":        "**type=...**\n\nSpecifies the data type for a parameter or field. Supports `string`, `int`, `float`, `bool`, `boolean`, `any`, and array suffixes (e.g., `string[]`).",
	"enum":        "**enum=a|b|c**\n\nRestricts a parameter or field to a specific set of allowed string values.",
	"default":     "**default=...**\n\nSpecifies the default value for a parameter if no value is provided during plan execution.",
	"title":       "**title=\"...\"**\n\nSpecifies a human-readable title for a parameter, used for UI display or documentation generation.",
	"require":     "**require <type> <name>**\n\nDeclares a global tool requirement for the plan. Supported types: `mcp`, `apicp`, `collection`, `function`, `search`.",
	"session":     "**session(\"name\", ...)**\n\nDefines a logical pipeline step (session). Each session can have its own context, tools, calls, and output schema.",
	"after":       "**after=\"session_name\"**\n\nSpecifies that this session should run after the named session. Acts as both an ordering constraint and a success gate.",
	"expect":      "**expect=expression**\n\nSpecifies a conditional expression that must evaluate to true for the session to execute.",
	"iterate":     "**iterate=expression**\n\nSpecifies an expression that returns a collection. The session will be executed once for each item in the collection.",
	"target":      "**target=\"name\"**\n\nOverrides the default property name in the root output schema for this session's results.",
	"use":         "**use <type> <name> [{ allowlist = [...] }]**\n\nDeclares a tool requirement for the session. Supported types: `mcp`, `apicp`, `collection`, `function`, `search`. An optional block can restrict available tools using an `allowlist`.",
	"allowlist":   "**allowlist = [\"tool1\", \"tool2\", ...]**\n\nRestricts the tools available from an MCP server, API collection, or other tool provider to a specific set of named tools.",
	"call":        "**call(\"name\") [-> [ns:]var] [{ ... }]**\n\nInvokes a tool or transformer. The output can be optionally mapped to a variable. The arguments block `{ ... }` is optional.",
	"resource":    "**resource \"identifier\" [-> [ns:]var]**\n\nSpecifies an external file or data source to be associated with the session. The resource can be optionally mapped to a variable.",
	"context":     "**context ...**\n\nSets the prompt context for the session. Can be a boolean or a string template.",
	"schema":      "**schema { ... }** or **schema Type**\n\nDefines the output structure for the session. Properties are merged into the session's object if using the `{}` syntax.",
	"schema?":     "**schema? ...**\n\nDefines an optional output structure. The session's property will not be marked as required in the root schema.",
	"set":         "**set var = value**\n\nDeclares a variable at the plan level or within a session.",
	"transformer": "**transformer(\"name\") { ... }**\n\nDefines a reusable output transformer that can be triggered by tool inputs or outputs.",
	"components":  "**components { ... }**\n\nDefines reusable schemas and prompt templates that can be referenced elsewhere in the plan.",
	"prompt":      "**prompt(\"name\") { \"...\" }**\n\nDefines a reusable prompt component within the `components` block.",
	"code":        "**code( ... )**\n\nExecutes custom JavaScript code for post-processing tool results or transformer logic.",
	"+":           "**+ Pre-prompt line**\n\nDefines a pre-prompt for the session. Multiple pre-prompts are collected into a single `prePrompt` field in the compiled plan. Pre-prompts are typically used for providing context or few-shot examples.",
	"-":           "**- Prompt line**\n\nDefines the main prompt for the session. Each session can have at most one prompt line. The prompt line is usually the final instruction sent to the LLM.",
}

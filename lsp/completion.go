package lsp

import (
	"context"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
)

func (h *FMLHandler) Completion(ctx context.Context, params *lsp.CompletionParams) (*lsp.CompletionList, error) {
	text, ok := h.documents[params.TextDocument.URI]
	if !ok {
		return nil, nil
	}

	lines := strings.Split(text, "\n")
	if params.Position.Line >= len(lines) {
		return nil, nil
	}

	currentLine := lines[params.Position.Line]
	charPos := params.Position.Character
	if charPos > len(currentLine) {
		charPos = len(currentLine)
	}
	prefix := currentLine[:charPos]
	trimmedPrefix := strings.TrimSpace(prefix)

	// 0. Context awareness: no completions inside comments or prompts
	if strings.HasPrefix(trimmedPrefix, "#") {
		return &lsp.CompletionList{Items: []lsp.CompletionItem{}}, nil
	}
	if (strings.HasPrefix(trimmedPrefix, "+") || strings.HasPrefix(trimmedPrefix, "-")) && (len(trimmedPrefix) > 1 || strings.HasSuffix(prefix, " ")) {
		return &lsp.CompletionList{Items: []lsp.CompletionItem{}}, nil
	}

	// 0.1 Prompt continuation awareness
	if h.isInPromptContinuation(lines, params.Position.Line) {
		return &lsp.CompletionList{Items: []lsp.CompletionItem{}}, nil
	}

	blockType := h.findBlockType(lines, params.Position.Line)

	// 1. Tool types after "use" or "require"
	if strings.HasSuffix(trimmedPrefix, "use") || strings.HasSuffix(trimmedPrefix, "require") {
		return &lsp.CompletionList{Items: toolTypeCompletions()}, nil
	}

	// 2. Types after ":"
	if strings.HasSuffix(trimmedPrefix, ":") || strings.HasSuffix(trimmedPrefix, ": ") {
		return &lsp.CompletionList{Items: scalarTypeCompletions()}, nil
	}

	// 3. Session and Parameter Attributes
	if (strings.HasPrefix(trimmedPrefix, "session") || strings.HasPrefix(trimmedPrefix, "parameter")) &&
		strings.Contains(prefix, "(") && !strings.Contains(prefix, ")") {
		return &lsp.CompletionList{Items: attributeCompletions(trimmedPrefix)}, nil
	}

	// 4. Transformer Fields
	if blockType == "transformer" {
		return &lsp.CompletionList{Items: transformerFieldCompletions()}, nil
	}

	// 5. Parser values after "parser ="
	if strings.HasSuffix(trimmedPrefix, "parser =") || strings.HasSuffix(trimmedPrefix, "parser = ") {
		return &lsp.CompletionList{Items: parserValueCompletions()}, nil
	}

	// 6. Session-level keywords
	if blockType == "session" {
		return &lsp.CompletionList{Items: sessionKeywordCompletions()}, nil
	}

	// 6.1 Use block keywords
	if blockType == "use" {
		return &lsp.CompletionList{Items: useBlockCompletions()}, nil
	}

	// 6.2 Components block keywords
	if blockType == "components" {
		return &lsp.CompletionList{Items: componentsKeywordCompletions()}, nil
	}

	// 6.3 Call block keywords
	if blockType == "call" {
		return &lsp.CompletionList{Items: callBlockCompletions()}, nil
	}

	// 7. Top-level blocks
	if blockType == "top" {
		return &lsp.CompletionList{
			IsIncomplete: false,
			Items:        topLevelKeywordCompletions(),
		}, nil
	}

	return &lsp.CompletionList{Items: []lsp.CompletionItem{}}, nil
}

func toolTypeCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "mcp", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		{Label: "apicp", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		{Label: "collection", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		{Label: "function", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		{Label: "search", Kind: ptr(lsp.CompletionItemKindEnumMember)},
	}
}

func scalarTypeCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "string", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
		{Label: "int", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
		{Label: "float", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
		{Label: "bool", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
		{Label: "boolean", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
		{Label: "any", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
	}
}

func attributeCompletions(prefix string) []lsp.CompletionItem {
	if strings.HasPrefix(prefix, "session") {
		return []lsp.CompletionItem{
			{Label: "after=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., after=\"...\")"},
			{Label: "expect=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., expect=...)"},
			{Label: "iterate=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., iterate=...)"},
			{Label: "target=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., target=\"...\")"},
		}
	}
	return []lsp.CompletionItem{
		{Label: "type=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., type=...)"},
		{Label: "default=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., default=...)"},
		{Label: "title=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., title=\"...\")"},
		{Label: "enum=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., enum=...)"},
	}
}

func transformerFieldCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "onFunctionOutput =", Kind: ptr(lsp.CompletionItemKindProperty)},
		{Label: "onFunctionInput =", Kind: ptr(lsp.CompletionItemKindProperty)},
		{Label: "onResource =", Kind: ptr(lsp.CompletionItemKindProperty)},
		{Label: "jmesPath =", Kind: ptr(lsp.CompletionItemKindProperty)},
		{Label: "parser =", Kind: ptr(lsp.CompletionItemKindProperty)},
		{Label: "code", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "code( ... )", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "JS post-processing code."}},
	}
}

func parserValueCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "json", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		{Label: "csv", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		{Label: "\"json\"", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		{Label: "\"csv\"", Kind: ptr(lsp.CompletionItemKindEnumMember)},
	}
}

func sessionKeywordCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "use", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "use <type> <name>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a tool requirement for the session."}},
		{Label: "call", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "call(\"...\") [-> var] [{ ... }]", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Invokes a tool or function. The fields block is optional."}},
		{Label: "resource", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "resource \"...\" [-> [ns:]var]", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Specifies an external file or data source to be associated with the session."}},
		{Label: "context", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "context ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Sets the session context."}},
		{Label: "schema", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "schema { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines the output structure for the session."}},
		{Label: "schema?", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "schema? { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines an optional output structure for the session."}},
		{Label: "set", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "set var = ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a variable."}},
		{Label: "+", Kind: ptr(lsp.CompletionItemKindSnippet), Detail: "+ <pre-prompt>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Starts a pre-prompt line."}},
		{Label: "-", Kind: ptr(lsp.CompletionItemKindSnippet), Detail: "- <prompt>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Starts the main prompt line."}},
	}
}

func useBlockCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "allowlist =", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "allowlist = [\"...\"]", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Restricts the tools available from a server or collection."}},
	}
}

func componentsKeywordCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "schema", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "schema(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable output schema."}},
		{Label: "prompt", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "prompt(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable prompt template."}},
		{Label: "script", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "script(\"...\", type=\"...\") ( ... )", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable script component."}},
	}
}

func callBlockCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "code", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "code( ... )", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Custom JavaScript code for post-processing tool results."}},
		{Label: "kbs", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "kbs( ... )", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Specifies knowledge base settings or references for the call."}},
	}
}

func topLevelKeywordCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "system", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "system(\"...\") or system(`...`)", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Sets the global system prompt. Supports multi-line backticks."}},
		{Label: "parameter", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "parameter(\"...\")", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines an input parameter for the plan."}},
		{Label: "transformer", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "transformer(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable output transformer."}},
		{Label: "require", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "require <type> <name>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a global tool requirement for the plan."}},
		{Label: "session", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "session(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a logical pipeline step."}},
		{Label: "components", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "components { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines reusable schemas and prompts."}},
		{Label: "set", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "set var = ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a variable."}},
	}
}

func (h *FMLHandler) isInPromptContinuation(lines []string, lineNum int) bool {
	if lineNum < 0 || lineNum >= len(lines) {
		return false
	}

	// Find current line's indentation
	currentLine := lines[lineNum]
	currentIndent := 0
	for _, r := range currentLine {
		if r == ' ' || r == '\t' {
			currentIndent++
		} else {
			break
		}
	}

	// If current line is blank, we use a virtual indent that would satisfy continuation
	// if we are indeed inside a prompt. getActivePromptMarker will tell us.
	trimmed := strings.TrimSpace(currentLine)
	isCurrentBlank := trimmed == ""

	mCol, ok := h.getActivePromptMarker(lines, lineNum-1)
	if !ok {
		return false
	}

	if isCurrentBlank {
		return true
	}

	return currentIndent > mCol
}

func (h *FMLHandler) findBlockType(lines []string, lineNum int) string {
	currentLine := lines[lineNum]
	trimmed := strings.TrimSpace(currentLine)

	// If current line is not indented, it's top-level context (unless it's an empty line)
	if len(currentLine) > 0 && currentLine[0] != ' ' && currentLine[0] != '\t' && trimmed != "" {
		return "top"
	}

	// Scan backwards to find the nearest non-indented line or nested block opener
	for i := lineNum - 1; i >= 0; i-- {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		// Check for nested block start on the lines above
		if strings.HasSuffix(trimmedLine, "{") {
			if strings.HasPrefix(trimmedLine, "use") {
				return "use"
			}
			if strings.HasPrefix(trimmedLine, "transformer") {
				return "transformer"
			}
			if strings.HasPrefix(trimmedLine, "session") {
				return "session"
			}
			if strings.HasPrefix(trimmedLine, "components") {
				return "components"
			}
			if strings.HasPrefix(trimmedLine, "call") {
				return "call"
			}
		}

		// If we find a non-indented line, it's a top-level block header
		if line[0] != ' ' && line[0] != '\t' {
			if strings.HasPrefix(trimmedLine, "session") {
				return "session"
			}
			if strings.HasPrefix(trimmedLine, "transformer") {
				return "transformer"
			}
			if strings.HasPrefix(trimmedLine, "components") {
				return "components"
			}
			return "top"
		}
	}

	return "top"
}

package main

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
	if int(params.Position.Line) >= len(lines) {
		return nil, nil
	}

	currentLine := lines[params.Position.Line]
	charPos := int(params.Position.Character)
	if charPos > len(currentLine) {
		charPos = len(currentLine)
	}
	prefix := currentLine[:charPos]
	trimmedPrefix := strings.TrimSpace(prefix)

	var items []lsp.CompletionItem

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

	// 3.1 Transformer Fields
	if strings.HasPrefix(trimmedPrefix, "transformer") || (len(prefix) > 0 && (prefix[0] == ' ' || prefix[0] == '\t')) {
		items = append(items, transformerFieldCompletions()...)
	}

	// 3.2 Parser values after "parser ="
	if strings.HasSuffix(trimmedPrefix, "parser =") || strings.HasSuffix(trimmedPrefix, "parser = ") {
		return &lsp.CompletionList{Items: parserValueCompletions()}, nil
	}

	// 4. Session-level keywords (indented or inside a block)
	if len(prefix) > 0 && (prefix[0] == ' ' || prefix[0] == '\t') {
		return &lsp.CompletionList{Items: sessionKeywordCompletions()}, nil
	}

	// 5. Top-level blocks (not indented)
	return &lsp.CompletionList{
		IsIncomplete: false,
		Items:        topLevelKeywordCompletions(),
	}, nil
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
		{Label: "context", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "context ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Sets the session context."}},
		{Label: "schema", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "schema { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines the output structure for the session."}},
		{Label: "schema?", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "schema? { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines an optional output structure for the session."}},
		{Label: "set", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "set var = ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a variable."}},
		{Label: "+", Kind: ptr(lsp.CompletionItemKindSnippet), Detail: "+ <pre-prompt>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Starts a pre-prompt line."}},
		{Label: "-", Kind: ptr(lsp.CompletionItemKindSnippet), Detail: "- <prompt>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Starts the main prompt line."}},
	}
}

func topLevelKeywordCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "system", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "system(\"...\")", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Sets the global system prompt."}},
		{Label: "parameter", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "parameter(\"...\")", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines an input parameter for the plan."}},
		{Label: "transformer", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "transformer(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable output transformer."}},
		{Label: "require", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "require <type> <name>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a global tool requirement for the plan."}},
		{Label: "session", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "session(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a logical pipeline step."}},
		{Label: "components", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "components { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines reusable schemas and prompts."}},
		{Label: "set", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "set var = ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a variable."}},
	}
}

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

	blockType := h.findBlockType(text, lines, params.Position.Line, int(params.Position.Character))

	// 1. Tool types after "use" or "require"
	if strings.HasSuffix(trimmedPrefix, "use") || strings.HasSuffix(trimmedPrefix, "require") {
		return &lsp.CompletionList{Items: toolTypeCompletions()}, nil
	}

	// 2. Types after ":"
	if strings.HasSuffix(trimmedPrefix, ":") || strings.HasSuffix(trimmedPrefix, ": ") {
		return &lsp.CompletionList{Items: scalarTypeCompletions()}, nil
	}

	// 3. Session, Parameter, and Script Attributes
	if (strings.HasPrefix(trimmedPrefix, "session") || strings.HasPrefix(trimmedPrefix, "parameter") || strings.HasPrefix(trimmedPrefix, "script")) &&
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
			{Label: "after=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., after=\"...\") or after=`...`"},
			{Label: "expect=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., expect=...)"},
			{Label: "iterate=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., iterate=...)"},
			{Label: "target=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., target=\"...\") or target=`...`"},
		}
	}
	if strings.HasPrefix(prefix, "script") {
		return []lsp.CompletionItem{
			{Label: "type=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "script(..., type=\"...\") or type=`...`"},
			{Label: "description=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "script(..., description=\"...\") or description=`...`"},
			{Label: "parameters=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "script(..., parameters={...})"},
		}
	}
	return []lsp.CompletionItem{
		{Label: "type=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., type=...)"},
		{Label: "default=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., default=...)"},
		{Label: "title=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., title=\"...\") or title=`...`"},
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
		{Label: "call", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "call(\"...\") or call(`...`)", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Invokes a tool or function. Supports single-line double-quotes or multi-line backticks."}},
		{Label: "resource", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "resource \"...\" or resource `...`", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Specifies an external file or data source to be associated with the session."}},
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
		{Label: "allowlist =", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "allowlist = [\"...\"] or allowlist = [`...`]", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Restricts the tools available from a server or collection."}},
	}
}

func componentsKeywordCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{Label: "schema", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "schema(\"...\") or schema(`...`)", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable output schema."}},
		{Label: "prompt", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "prompt(\"...\") or prompt(`...`)", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable prompt template."}},
		{Label: "script", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "script(\"...\", type=\"...\") or script(`...`, type=`...`)", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable script component."}},
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
		{Label: "parameter", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "parameter(\"...\") or parameter(`...`)", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines an input parameter for the plan. Supports double quotes or backticks."}},
		{Label: "transformer", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "transformer(\"...\") or transformer(`...`)", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable output transformer. Supports double quotes or backticks."}},
		{Label: "require", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "require <type> <name>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a global tool requirement for the plan."}},
		{Label: "session", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "session(\"...\") or session(`...`)", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a logical pipeline step. Supports double quotes or backticks."}},
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

func (h *FMLHandler) findBlockType(text string, lines []string, lineNum int, charNum int) string {
	currentLine := lines[lineNum]
	trimmed := strings.TrimSpace(currentLine)

	// If current line is not indented, it's top-level context (unless it's an empty line)
	// But if the cursor is inside parenthesis or braces on this line, let the state machine run instead.
	prefixUpToCursor := ""
	if charNum <= len(currentLine) {
		prefixUpToCursor = currentLine[:charNum]
	} else {
		prefixUpToCursor = currentLine
	}
	isInsideOpenerOnLine := (strings.Contains(prefixUpToCursor, "(") && !strings.Contains(prefixUpToCursor, ")")) ||
		(strings.Contains(prefixUpToCursor, "[") && !strings.Contains(prefixUpToCursor, "]")) ||
		(strings.Contains(prefixUpToCursor, "{") && !strings.Contains(prefixUpToCursor, "}"))

	if len(currentLine) > 0 && currentLine[0] != ' ' && currentLine[0] != '\t' && trimmed != "" && !isInsideOpenerOnLine {
		return "top"
	}

	// Calculate exact cursor offset in the full text using rune-aligned logic
	cursorOffset := 0
	for i := 0; i < lineNum; i++ {
		cursorOffset += len([]rune(lines[i])) + 1 // +1 for '\n'
	}
	cursorOffset += charNum

	type opener struct {
		char   rune
		offset int
		line   int
	}
	var stack []opener
	lineCount := 0

	inComment := false
	inString := false
	inRawString := false

	runes := []rune(text)
	// Run standard state machine forward from 0 to cursorOffset to find the active unclosed openers at cursor
	for offset := 0; offset < len(runes) && offset < cursorOffset; offset++ {
		r := runes[offset]

		if r == '\n' {
			lineCount++
			if inComment {
				inComment = false
			}
			continue
		}

		if inComment {
			continue
		}

		if inString {
			if r == '"' {
				escaped := false
				if offset > 0 && runes[offset-1] == '\\' {
					// Count backslashes
					bsCount := 0
					for k := offset - 1; k >= 0; k-- {
						if runes[k] == '\\' {
							bsCount++
						} else {
							break
						}
					}
					if bsCount%2 == 1 {
						escaped = true
					}
				}
				if !escaped {
					inString = false
				}
			}
			continue
		}

		if inRawString {
			if r == '`' {
				inRawString = false
			}
			continue
		}

		// Not in comment, string, or raw string
		if r == '#' {
			inComment = true
			continue
		}
		if r == '"' {
			inString = true
			continue
		}
		if r == '`' {
			inRawString = true
			continue
		}

		if r == '{' || r == '(' || r == '[' {
			stack = append(stack, opener{char: r, offset: offset, line: lineCount})
		} else if r == '}' || r == ')' || r == ']' {
			matching := ' '
			if r == '}' {
				matching = '{'
			} else if r == ')' {
				matching = '('
			} else if r == ']' {
				matching = '['
			}

			// Find matching opener from the end of stack
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k].char == matching {
					stack = stack[:k]
					break
				}
			}
		}
	}

	if len(stack) == 0 {
		return "top"
	}

	lastOpener := stack[len(stack)-1]
	if lastOpener.char == '(' || lastOpener.char == '[' {
		return "paren"
	}

	// It is '{'. Look at the line of the opener.
	if lastOpener.line >= len(lines) {
		return "top"
	}
	openerLine := lines[lastOpener.line]
	trimmedOpener := strings.TrimSpace(openerLine)

	// If the line containing '{' doesn't start with a known keyword, scan upwards to find the header line
	headerLine := trimmedOpener
	for k := lastOpener.line; k >= 0; k-- {
		trimmedL := strings.TrimSpace(lines[k])
		// Skip if empty or just comment
		if trimmedL == "" || strings.HasPrefix(trimmedL, "#") {
			continue
		}
		if strings.HasPrefix(trimmedL, "use") ||
			strings.HasPrefix(trimmedL, "transformer") ||
			strings.HasPrefix(trimmedL, "session") ||
			strings.HasPrefix(trimmedL, "components") ||
			strings.HasPrefix(trimmedL, "call") {
			headerLine = trimmedL
			break
		}
		// If we hit a different closed brace/bracket structure, stop
		if k < lastOpener.line && (strings.HasSuffix(trimmedL, "}") || strings.HasSuffix(trimmedL, ")")) {
			break
		}
	}

	if strings.HasPrefix(headerLine, "use") {
		return "use"
	}
	if strings.HasPrefix(headerLine, "transformer") {
		return "transformer"
	}
	if strings.HasPrefix(headerLine, "session") {
		return "session"
	}
	if strings.HasPrefix(headerLine, "components") {
		return "components"
	}
	if strings.HasPrefix(headerLine, "call") {
		return "call"
	}

	return "nested"
}

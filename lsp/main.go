package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
	"github.com/theirish/fml/parser"
)

type FMLHandler struct {
	client    *server.Client
	documents map[lsp.DocumentURI]string
}

func (h *FMLHandler) SetClient(client *server.Client) {
	h.client = client
	h.documents = make(map[lsp.DocumentURI]string)
}

func (h *FMLHandler) Initialize(ctx context.Context, params *lsp.InitializeParams) (*lsp.InitializeResult, error) {
	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptions{
				OpenClose: ptr(true),
				Change:    lsp.SyncFull,
			},
			CompletionProvider: &lsp.CompletionOptions{
				TriggerCharacters: []string{"(", ",", ":", "=", "$", " "},
			},
			HoverProvider: ptr(true),
			SemanticTokensProvider: &lsp.SemanticTokensOptions{
				Legend: lsp.SemanticTokensLegend{
					TokenTypes: []string{
						"keyword", "string", "number", "comment", "type", "parameter", "variable",
					},
					TokenModifiers: []string{},
				},
				Full: &lsp.SemanticTokensFull{
					Delta: ptr(false),
				},
			},
		},
		ServerInfo: &lsp.ServerInfo{
			Name:    "FML Language Server",
			Version: "0.1.0",
		},
	}, nil
}

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

	// 1. Tool types after "use"
	if strings.HasSuffix(trimmedPrefix, "use") {
		items = []lsp.CompletionItem{
			{Label: "mcp", Kind: ptr(lsp.CompletionItemKindEnumMember)},
			{Label: "apicp", Kind: ptr(lsp.CompletionItemKindEnumMember)},
			{Label: "search", Kind: ptr(lsp.CompletionItemKindEnumMember)},
			{Label: "function", Kind: ptr(lsp.CompletionItemKindEnumMember)},
			{Label: "collection", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		}
		return &lsp.CompletionList{Items: items}, nil
	}

	// 2. Types after ":"
	if strings.HasSuffix(trimmedPrefix, ":") || strings.HasSuffix(trimmedPrefix, ": ") {
		items = []lsp.CompletionItem{
			{Label: "string", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
			{Label: "int", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
			{Label: "float", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
			{Label: "bool", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
			{Label: "any", Kind: ptr(lsp.CompletionItemKindTypeParameter)},
		}
		return &lsp.CompletionList{Items: items}, nil
	}

	// 3. Session and Parameter Attributes
	if (strings.HasPrefix(trimmedPrefix, "session") || strings.HasPrefix(trimmedPrefix, "parameter")) &&
		strings.Contains(prefix, "(") && !strings.Contains(prefix, ")") {

		if strings.HasPrefix(trimmedPrefix, "session") {
			items = []lsp.CompletionItem{
				{Label: "after=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., after=\"...\")"},
				{Label: "expect=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., expect=...)"},
				{Label: "iterate=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., iterate=...)"},
				{Label: "target=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "session(..., target=\"...\")"},
			}
		} else {
			items = []lsp.CompletionItem{
				{Label: "type=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., type=...)"},
				{Label: "default=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., default=...)"},
				{Label: "title=", Kind: ptr(lsp.CompletionItemKindProperty), Detail: "parameter(..., title=\"...\")"},
			}
		}
		return &lsp.CompletionList{Items: items}, nil
	}

	// 3.1 Transformer Fields
	if strings.HasPrefix(trimmedPrefix, "transformer") || (len(prefix) > 0 && (prefix[0] == ' ' || prefix[0] == '\t')) {
		// Check if we are likely inside a transformer block
		// This is a heuristic as we don't have a full AST-based position resolver yet
		items = append(items, []lsp.CompletionItem{
			{Label: "onFunctionOutput =", Kind: ptr(lsp.CompletionItemKindProperty)},
			{Label: "onFunctionInput =", Kind: ptr(lsp.CompletionItemKindProperty)},
			{Label: "onResource =", Kind: ptr(lsp.CompletionItemKindProperty)},
			{Label: "jmesPath =", Kind: ptr(lsp.CompletionItemKindProperty)},
			{Label: "parser =", Kind: ptr(lsp.CompletionItemKindProperty)},
			{Label: "code", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "code( ... )", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "JS post-processing code."}},
		}...)
	}

	// 3.2 Parser values after "parser ="
	if strings.HasSuffix(trimmedPrefix, "parser =") || strings.HasSuffix(trimmedPrefix, "parser = ") {
		items = []lsp.CompletionItem{
			{Label: "json", Kind: ptr(lsp.CompletionItemKindEnumMember)},
			{Label: "csv", Kind: ptr(lsp.CompletionItemKindEnumMember)},
			{Label: "\"json\"", Kind: ptr(lsp.CompletionItemKindEnumMember)},
			{Label: "\"csv\"", Kind: ptr(lsp.CompletionItemKindEnumMember)},
		}
		return &lsp.CompletionList{Items: items}, nil
	}

	// 4. Session-level keywords (indented or inside a block)
	if len(prefix) > 0 && (prefix[0] == ' ' || prefix[0] == '\t') {
		items = []lsp.CompletionItem{
			{Label: "use", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "use <type> <name>", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a tool requirement for the session."}},
			{Label: "call", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "call(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Invokes a tool or function."}},
			{Label: "context", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "context ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Sets the session context."}},
			{Label: "schema", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "schema { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines the output structure for the session."}},
			{Label: "schema?", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "schema? { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines an optional output structure for the session."}},
			{Label: "set", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "set var = ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a variable."}},
		}
		return &lsp.CompletionList{Items: items}, nil
	}

	// 5. Top-level blocks (not indented)
	items = []lsp.CompletionItem{
		{Label: "system", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "system(\"...\")", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Sets the global system prompt."}},
		{Label: "parameter", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "parameter(\"...\")", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines an input parameter for the plan."}},
		{Label: "transformer", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "transformer(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a reusable output transformer."}},
		{Label: "session", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "session(\"...\") { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines a logical pipeline step."}},
		{Label: "components", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "components { ... }", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Defines reusable schemas and prompts."}},
		{Label: "set", Kind: ptr(lsp.CompletionItemKindKeyword), Detail: "set var = ...", Documentation: &lsp.MarkupContent{Kind: lsp.Markdown, Value: "Declares a variable."}},
	}

	return &lsp.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil
}

func (h *FMLHandler) Shutdown(ctx context.Context) error {
	return nil
}

func (h *FMLHandler) DidOpen(ctx context.Context, params *lsp.DidOpenTextDocumentParams) error {
	h.documents[params.TextDocument.URI] = params.TextDocument.Text
	h.diagnose(ctx, params.TextDocument.URI, params.TextDocument.Text)
	return nil
}

func (h *FMLHandler) DidChange(ctx context.Context, params *lsp.DidChangeTextDocumentParams) error {
	if len(params.ContentChanges) > 0 {
		text := params.ContentChanges[len(params.ContentChanges)-1].Text
		h.documents[params.TextDocument.URI] = text
		h.diagnose(ctx, params.TextDocument.URI, text)
	}
	return nil
}

func (h *FMLHandler) DidClose(ctx context.Context, params *lsp.DidCloseTextDocumentParams) error {
	delete(h.documents, params.TextDocument.URI)
	return nil
}

func (h *FMLHandler) SemanticTokensFull(ctx context.Context, params *lsp.SemanticTokensParams) (*lsp.SemanticTokens, error) {
	text, ok := h.documents[params.TextDocument.URI]
	if !ok {
		return nil, nil
	}

	lexerDef := &parser.FRAGSLexerDefinition{}
	l, err := lexerDef.Lex(string(params.TextDocument.URI), strings.NewReader(text))
	if err != nil {
		return nil, nil
	}

	data := []int{}
	lastLine := 0
	lastChar := 0

	keywords := map[string]int{
		"system": 0, "parameter": 0, "transformer": 0, "call": 0, "session": 0,
		"use": 0, "mcp": 0, "apicp": 0, "collection": 0, "function": 0, "search": 0,
		"context": 0, "set": 0, "schema": 0, "components": 0, "prompt": 0, "code": 0,
		"after": 0, "expect": 0, "iterate": 0, "target": 0, "type": 0, "default": 0, "title": 0,
		"onFunctionOutput": 0, "onFunctionInput": 0, "onResource": 0, "jmesPath": 0, "parser": 0,
		"true": 0, "false": 0,
		"string": 4, "int": 4, "float": 4, "bool": 4, "any": 4,
		"json": 1, "csv": 1,
	}

	for {
		t, err := l.Next()
		if err != nil || t.Type == -1 {
			break
		}

		tokenType := -1
		switch t.Type {
		case -2, -10: // Comment, InlineComment
			tokenType = 3
		case -3, -9: // String, PromptItem
			tokenType = 1
		case -4: // Number
			tokenType = 2
		case -5: // Bool
			tokenType = 0
		case -6: // Ident
			if idx, ok := keywords[t.Value]; ok {
				tokenType = idx
			} else {
				tokenType = 6 // variable/parameter as fallback
			}
		case -11, -12: // AttrValue, CodeValue
			tokenType = 1 // Highlighting raw values as strings for now
		}

		if tokenType != -1 {
			line := t.Pos.Line - 1
			char := t.Pos.Column - 1

			lines := strings.Split(t.Value, "\n")
			for i, lineText := range lines {
				dLine := 0
				dChar := 0
				if i == 0 {
					dLine = line - lastLine
					if dLine == 0 {
						dChar = char - lastChar
					} else {
						dChar = char
					}
				} else {
					dLine = 1
					dChar = 0
				}

				length := len(strings.TrimRight(lineText, "\r"))
				if length > 0 {
					data = append(data, dLine, dChar, length, tokenType, 0)
					lastLine += dLine
					if dLine == 0 {
						lastChar += dChar
					} else {
						lastChar = dChar
					}
				} else if i > 0 {
					// Account for the newline even if the line is empty
					lastLine += 1
					lastChar = 0
				}
			}
		}
	}

	return &lsp.SemanticTokens{
		Data: data,
	}, nil
}

func (h *FMLHandler) diagnose(ctx context.Context, uri lsp.DocumentURI, text string) {
	diagnostics := make([]lsp.Diagnostic, 0)

	p, err := parser.NewParser()
	if err == nil {
		_, err = p.ParseString(string(uri), text)
		if err != nil {
			msg := err.Error()
			severity := lsp.SeverityError
			diag := lsp.Diagnostic{
				Severity: &severity,
				Message:  msg,
				Range: lsp.Range{
					Start: lsp.Position{Line: 0, Character: 0},
					End:   lsp.Position{Line: 0, Character: 0},
				},
			}

			// Try to parse line and column from the error message
			// Format: <filename>:<line>:<col>: <message>
			parts := strings.SplitN(msg, ":", 4)
			if len(parts) >= 3 {
				var line, col int
				_, errLine := fmt.Sscanf(parts[1], "%d", &line)
				_, errCol := fmt.Sscanf(parts[2], "%d", &col)
				if errLine == nil && errCol == nil {
					// LSP uses 0-based indexing
					pos := lsp.Position{Line: line - 1, Character: col - 1}
					diag.Range = lsp.Range{Start: pos, End: pos}
					if len(parts) > 3 {
						diag.Message = strings.TrimSpace(parts[3])
					}
				}
			}
			diagnostics = append(diagnostics, diag)
		}
	}

	if h.client != nil {
		_ = h.client.PublishDiagnostics(ctx, &lsp.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diagnostics,
		})
	}
}

func (h *FMLHandler) Hover(ctx context.Context, params *lsp.HoverParams) (*lsp.Hover, error) {
	// Simple keyword-based hover for demonstration
	// In a real implementation, we would use the AST to find the symbol at the position

	// For now, let's just return a placeholder or leave it as an exercise.
	return &lsp.Hover{
		Contents: lsp.MarkupContent{
			Kind:  lsp.Markdown,
			Value: "FML Language Server: Hover support active.",
		},
	}, nil
}

func ptr[T any](v T) *T {
	return &v
}

func main() {
	handler := &FMLHandler{}
	srv := server.NewServer(handler, server.WithLogger(slog.Default()))

	if err := srv.Run(context.Background(), server.RunStdio()); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

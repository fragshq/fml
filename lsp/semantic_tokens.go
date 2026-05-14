package main

import (
	"context"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/theirish81/fml/parser"
)

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

	data := make([]int, 0)
	lastLine := 0
	lastChar := 0

	for {
		t, err := l.Next()
		if err != nil || t.Type == -1 {
			break
		}

		tokenType := -1
		switch t.Type {
		case -2, -10: // Comment, InlineComment
			tokenType = 3
		case -3, -9, -13: // String, PromptItem, PrePromptItem
			tokenType = 1
		case -4: // Number
			tokenType = 2
		case -5: // Bool
			tokenType = 0
		case -6: // Ident
			if idx, ok := semanticKeywords[t.Value]; ok {
				tokenType = idx
			} else {
				tokenType = 6 // variable/parameter as fallback
			}
		case -11, -12: // AttrValue, CodeValue
			tokenType = 1 // Highlighting raw values as strings for now
		}

		if tokenType != -1 {
			lines := strings.Split(t.Value, "\n")
			for i, lineText := range lines {
				length := len(strings.TrimRight(lineText, "\r"))
				if length == 0 {
					continue
				}

				dLine := 0
				dChar := 0
				if i == 0 {
					dLine = (t.Pos.Line + i - 1) - lastLine
					if dLine == 0 {
						dChar = (t.Pos.Column - 1) - lastChar
					} else {
						dChar = t.Pos.Column - 1
					}
				} else {
					dLine = 1
					dChar = 0 // Start from column 1 for continuation lines
				}

				data = append(data, dLine, dChar, length, tokenType, 0)
				lastLine += dLine
				if dLine == 0 {
					lastChar += dChar
				} else {
					lastChar = dChar
				}
			}
		}
	}

	return &lsp.SemanticTokens{
		Data: data,
	}, nil
}

var semanticKeywords = map[string]int{
	"system": 0, "parameter": 0, "transformer": 0, "call": 0, "session": 0,
	"require": 0, "use": 0, "mcp": 0, "apicp": 0, "collection": 0, "function": 0, "search": 0,
	"context": 0, "set": 0, "schema": 0, "components": 0, "prompt": 0, "code": 0, "resource": 0,
	"after": 0, "expect": 0, "iterate": 0, "target": 0, "type": 0, "default": 0, "title": 0,
	"allowlist":        0,
	"onFunctionOutput": 0, "onFunctionInput": 0, "onResource": 0, "jmesPath": 0, "parser": 0,
	"true": 0, "false": 0,
	"string": 4, "int": 4, "float": 4, "bool": 4, "any": 4,
	"json": 1, "csv": 1,
}

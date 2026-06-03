package lsp

import (
	"context"
	"strings"
	"unicode/utf16"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/theirish81/fml/parser"
)

type semanticToken struct {
	line      int
	char      int
	length    int
	tokenType int
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

	var tokens []semanticToken

	for {
		t, err := l.Next()
		if err != nil || t.Type == -1 {
			break
		}

		tokenType := -1
		switch t.Type {
		case -2, -10: // Comment, InlineComment
			tokenType = 3
		case -3, -14, -9, -13: // String, RawString, PromptItem, PrePromptItem
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
			// Split multi-line tokens into absolute single-line tokens
			lines := strings.Split(t.Value, "\n")
			for i, lineText := range lines {
				trimmedLine := strings.TrimRight(lineText, "\r")
				u16 := utf16.Encode([]rune(trimmedLine))
				length := len(u16)

				if length > 0 {
					absLine := t.Pos.Line + i - 1
					absChar := 0
					if i == 0 {
						// For the first line, the column comes from the lexer.
						// BUT we must account for any multi-byte characters BEFORE the token on the same line?
						// No, the lexer's Pos.Column is now correct (rune-based).
						absChar = t.Pos.Column - 1
					} else {
						// Continuation lines start at column 0 in the token value
						absChar = 0
					}

					tokens = append(tokens, semanticToken{
						line:      absLine,
						char:      absChar,
						length:    length,
						tokenType: tokenType,
					})
				}
			}
		}
	}

	// Convert absolute tokens to relative data
	data := make([]int, 0, len(tokens)*5)
	lastLine := 0
	lastChar := 0

	for _, t := range tokens {
		dLine := t.line - lastLine
		dChar := t.char
		if dLine == 0 {
			dChar = t.char - lastChar
		}

		data = append(data, dLine, dChar, t.length, t.tokenType, 0)
		lastLine = t.line
		lastChar = t.char
	}

	return &lsp.SemanticTokens{
		Data: data,
	}, nil
}

var semanticKeywords = map[string]int{
	"system": 0, "parameter": 0, "transformer": 0, "call": 0, "session": 0,
	"require": 0, "use": 0, "mcp": 0, "apicp": 0, "collection": 0, "function": 0, "search": 0,
	"context": 0, "set": 0, "vars": 0, "schema": 0, "components": 0, "prompt": 0, "code": 0, "resource": 0,
	"after": 0, "expect": 0, "iterate": 0, "target": 0, "type": 0, "default": 0, "title": 0, "enum": 0,
	"allowlist":        0,
	"onFunctionOutput": 0, "onFunctionInput": 0, "onResource": 0, "jmesPath": 0, "parser": 0,
	"true": 0, "false": 0,
	"string": 0, "int": 0, "integer": 0, "float": 0, "number": 0, "bool": 0, "boolean": 0, "any": 0,
	"object": 0, "array": 0,
	"json": 1, "csv": 1,
}

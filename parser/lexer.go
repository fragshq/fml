package parser

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/alecthomas/participle/v2/lexer"
)

// FRAGSLexerDefinition implements lexer.Definition for the Frags DSL.
type FRAGSLexerDefinition struct{}

func (d *FRAGSLexerDefinition) Lex(filename string, r io.Reader) (lexer.Lexer, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &fragsLexer{
		s:   string(b),
		pos: lexer.Position{Filename: filename, Line: 1, Column: 1},
	}, nil
}

func (d *FRAGSLexerDefinition) Symbols() map[string]lexer.TokenType {
	return map[string]lexer.TokenType{
		"Comment":       -2,
		"InlineComment": -10,
		"String":        -3,
		"Number":        -4,
		"Bool":          -5,
		"Ident":         -6,
		"Punct":         -7,
		"Whitespace":    -8,
		"PromptItem":    -9,
		"PrePromptItem": -13,
		"AttrValue":     -11,
		"CodeValue":     -12,
		"EOF":           -1,
	}
}

// fragsLexer is a stateful lexer that handles indentation-sensitive prompt blocks,
// balanced-parentheses code segments, and robust error detection.
type fragsLexer struct {
	s                  string
	pos                lexer.Position
	expectingAttrValue bool
	expectingCode      bool
	lastIdent          string
}

func (l *fragsLexer) Next() (lexer.Token, error) {
	for {
		if l.pos.Column == 1 {
			i := 0
			for i < len(l.s) && (l.s[i] == ' ' || l.s[i] == '\t') {
				i++
			}
			isPrompt := i < len(l.s) && l.s[i] == '-' && (i+1 == len(l.s) || l.s[i+1] == ' ' || l.s[i+1] == '\n' || l.s[i+1] == '\r')
			isPrePrompt := i < len(l.s) && l.s[i] == '+' && (i+1 == len(l.s) || l.s[i+1] == ' ' || l.s[i+1] == '\n' || l.s[i+1] == '\r')

			if isPrompt || isPrePrompt {
				t, err := l.consumePromptItem(i, isPrePrompt)
				return t, err
			}

			if i < len(l.s) && l.s[i] == '#' {
				l.s = l.s[i:]
				l.pos.Column += i
				return l.consumeComment("Comment"), nil
			}
		}

		i := 0
		for i < len(l.s) && (l.s[i] == ' ' || l.s[i] == '\t') {
			i++
		}
		if i > 0 {
			l.s = l.s[i:]
			l.pos.Column += i
		}

		if len(l.s) == 0 {
			return lexer.EOFToken(l.pos), nil
		}

		if l.s[0] == '\n' || l.s[0] == '\r' {
			l.consumeNewline()
			l.lastIdent = ""
			continue
		}
		break
	}

	if l.expectingAttrValue {
		l.expectingAttrValue = false
		if l.s[0] != '"' {
			i := 0
			for i < len(l.s) && l.s[i] != ',' && l.s[i] != ')' && l.s[i] != '\n' && l.s[i] != '\r' {
				i++
			}
			val := strings.TrimSpace(l.s[:i])
			t := l.consume(i, "AttrValue")
			t.Value = val
			return t, nil
		}
	}

	if l.expectingCode {
		l.expectingCode = false
		t, err := l.consumeBalanced('(', ')')
		if err != nil {
			return t, err
		}
		t.Type = -12 // CodeValue
		return t, nil
	}

	var t lexer.Token
	var err error
	{
		r := l.s[0]
		switch {
		case r == '#':
			t = l.consumeComment("InlineComment")
		case r == '"':
			t, err = l.consumeString()
			l.lastIdent = ""
		case unicode.IsDigit(rune(r)) || r == '-' || r == '+':
			if r == '-' {
				if strings.HasPrefix(l.s, "->") {
					t = l.consume(2, "Punct")
				} else if len(l.s) > 1 && unicode.IsDigit(rune(l.s[1])) {
					t, err = l.consumeNumber()
				} else {
					t = l.consume(1, "Punct")
				}
			} else {
				t, err = l.consumeNumber()
			}
			l.lastIdent = ""
		case unicode.IsLetter(rune(r)) || r == '_':
			t = l.consumeIdent()
			l.lastIdent = t.Value
		case strings.ContainsRune("][{}():,=|?$-", rune(r)):
			if strings.HasPrefix(l.s, "->") {
				t = l.consume(2, "Punct")
				l.lastIdent = ""
			} else if strings.HasPrefix(l.s, "$(") {
				t = l.consume(2, "Punct")
				l.expectingCode = true
				l.lastIdent = ""
			} else {
				t = l.consume(1, "Punct")
				if t.Value == "=" {
					if l.lastIdent == "expect" || l.lastIdent == "iterate" || l.lastIdent == "after" {
						l.expectingAttrValue = true
					}
				} else if t.Value == "(" {
					if l.lastIdent == "code" {
						l.expectingCode = true
					}
				}
				if t.Value != "," {
					l.lastIdent = ""
				}
			}
		default:
			return lexer.Token{}, fmt.Errorf("%s: illegal character %q", l.pos, r)
		}
	}

	return t, err
}

func (l *fragsLexer) consumeBalanced(start, end rune) (lexer.Token, error) {
	depth := 1
	i := 0
	startPos := l.pos

	for i < len(l.s) {
		r := rune(l.s[i])
		if r == start {
			depth++
		} else if r == end {
			depth--
			if depth == 0 {
				break
			}
		}
		i++
	}

	if depth > 0 {
		return lexer.Token{}, fmt.Errorf("%s: unclosed balanced block (missing %q)", startPos, end)
	}

	val := l.s[:i]
	l.s = l.s[i:]
	for _, r := range val {
		if r == '\n' {
			l.pos.Line++
			l.pos.Column = 1
		} else {
			l.pos.Column++
		}
	}

	return lexer.Token{
		Type:  -7,
		Value: val,
		Pos:   startPos,
	}, nil
}

func (l *fragsLexer) consumeNewline() {
	if strings.HasPrefix(l.s, "\r\n") {
		l.s = l.s[2:]
	} else {
		l.s = l.s[1:]
	}
	l.pos.Line++
	l.pos.Column = 1
}

func (l *fragsLexer) consume(n int, typ string) lexer.Token {
	val := l.s[:n]
	token := lexer.Token{
		Type:  l.typeToToken(typ),
		Value: val,
		Pos:   l.pos,
	}
	l.s = l.s[n:]
	l.pos.Column += n
	return token
}

func (l *fragsLexer) typeToToken(typ string) lexer.TokenType {
	switch typ {
	case "Comment":
		return -2
	case "InlineComment":
		return -10
	case "String":
		return -3
	case "Number":
		return -4
	case "Bool":
		return -5
	case "Ident":
		return -6
	case "Punct":
		return -7
	case "Whitespace":
		return -8
	case "PromptItem":
		return -9
	case "PrePromptItem":
		return -13
	case "AttrValue":
		return -11
	case "CodeValue":
		return -12
	}
	return -1
}

func (l *fragsLexer) consumeComment(typ string) lexer.Token {
	i := 0
	for i < len(l.s) && l.s[i] != '\n' && l.s[i] != '\r' {
		i++
	}
	return l.consume(i, typ)
}

func (l *fragsLexer) consumeString() (lexer.Token, error) {
	startPos := l.pos
	i := 1
	for i < len(l.s) {
		if l.s[i] == '\n' || l.s[i] == '\r' {
			return lexer.Token{}, fmt.Errorf("%s: unterminated string literal", startPos)
		}
		if l.s[i] == '\\' {
			if i+1 >= len(l.s) {
				return lexer.Token{}, fmt.Errorf("%s: unterminated string literal", startPos)
			}
			esc := l.s[i+1]
			if esc != '"' && esc != '\\' && esc != 'n' && esc != 'r' && esc != 't' {
				return lexer.Token{}, fmt.Errorf("%s: invalid escape sequence \\%c", l.pos, esc)
			}
			i += 2
			continue
		}
		if l.s[i] == '"' {
			val := l.s[:i+1]
			// Validate template tags {{ }} are balanced
			if strings.Count(val, "{{") != strings.Count(val, "}}") {
				return lexer.Token{}, fmt.Errorf("%s: malformed template tags in string", startPos)
			}
			i++
			return l.consume(i, "String"), nil
		}
		i++
	}
	return lexer.Token{}, fmt.Errorf("%s: unterminated string literal", startPos)
}

func (l *fragsLexer) consumeNumber() (lexer.Token, error) {
	startPos := l.pos
	i := 0
	if l.s[i] == '-' || l.s[i] == '+' {
		i++
	}
	dots := 0
	for i < len(l.s) && (unicode.IsDigit(rune(l.s[i])) || l.s[i] == '.') {
		if l.s[i] == '.' {
			dots++
			if dots > 1 {
				return lexer.Token{}, fmt.Errorf("%s: malformed number (multiple decimal points)", startPos)
			}
		}
		i++
	}
	if l.s[i-1] == '.' {
		return lexer.Token{}, fmt.Errorf("%s: malformed number (trailing decimal point)", startPos)
	}
	return l.consume(i, "Number"), nil
}

func (l *fragsLexer) consumeIdent() lexer.Token {
	i := 0
	for i < len(l.s) && (unicode.IsLetter(rune(l.s[i])) || unicode.IsDigit(rune(l.s[i])) || l.s[i] == '_') {
		i++
	}
	val := l.s[:i]
	if val == "true" || val == "false" {
		return l.consume(i, "Bool")
	}
	return l.consume(i, "Ident")
}

func (l *fragsLexer) consumePromptItem(indent int, isPrePrompt bool) (lexer.Token, error) {
	startPos := l.pos
	dashCol := indent + 1
	i := 0
	for i < len(l.s) && l.s[i] != '\n' && l.s[i] != '\r' {
		i++
	}

	totalLen := i
	for {
		nextStart := totalLen
		if nextStart >= len(l.s) {
			break
		}
		j := nextStart
		if strings.HasPrefix(l.s[j:], "\r\n") {
			j += 2
		} else if l.s[j] == '\n' || l.s[j] == '\r' {
			j++
		} else {
			break
		}

		k := 0
		for j+k < len(l.s) && (l.s[j+k] == ' ' || l.s[j+k] == '\t') {
			k++
		}

		if j+k < len(l.s) && l.s[j+k] != '\n' && l.s[j+k] != '\r' && k > dashCol {
			j += k
			for j < len(l.s) && l.s[j] != '\n' && l.s[j] != '\r' {
				j++
			}
			totalLen = j
			continue
		}
		break
	}

	val := l.s[:totalLen]
	// Validate template tags
	if strings.Count(val, "{{") != strings.Count(val, "}}") {
		return lexer.Token{}, fmt.Errorf("%s: malformed template tags in prompt item", startPos)
	}

	typ := -9
	if isPrePrompt {
		typ = -13
	}

	token := lexer.Token{
		Type:  lexer.TokenType(typ),
		Value: val[indent:],
		Pos:   lexer.Position{Filename: startPos.Filename, Line: startPos.Line, Column: startPos.Column + indent},
	}

	l.s = l.s[totalLen:]
	for _, r := range val {
		if r == '\n' {
			l.pos.Line++
			l.pos.Column = 1
		} else {
			l.pos.Column++
		}
	}

	return token, nil
}

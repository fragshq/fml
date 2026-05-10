package parser

import (
	"io"
	"strings"
	"unicode"

	"github.com/alecthomas/participle/v2/lexer"
)

// fragsLexerDefinition implements lexer.Definition for the Frags DSL.
type fragsLexerDefinition struct{}

func (d *fragsLexerDefinition) Lex(filename string, r io.Reader) (lexer.Lexer, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &fragsLexer{
		s:   string(b),
		pos: lexer.Position{Filename: filename, Line: 1, Column: 1},
	}, nil
}

func (d *fragsLexerDefinition) Symbols() map[string]lexer.TokenType {
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
		"AttrValue":     -11,
		"CodeValue":     -12,
		"EOF":           -1,
	}
}

// fragsLexer is a stateful lexer that handles indentation-sensitive prompt blocks
// and balanced-parentheses code segments.
type fragsLexer struct {
	s                  string
	pos                lexer.Position
	expectingAttrValue bool   // Set when encountering '=' after after/expect/iterate
	expectingCode      bool   // Set when encountering 'code('
	lastIdent          string // Tracks the last identifier to contextually trigger special states
}

func (l *fragsLexer) Next() (lexer.Token, error) {
	for {
		// Prompt items are detected at the start of a line to capture their specific indentation.
		if l.pos.Column == 1 {
			i := 0
			for i < len(l.s) && (l.s[i] == ' ' || l.s[i] == '\t') {
				i++
			}
			// Check for dash line marker '- '
			if i < len(l.s) && l.s[i] == '-' && (i+1 == len(l.s) || l.s[i+1] == ' ' || l.s[i+1] == '\n' || l.s[i+1] == '\r') {
				return l.consumePromptItem(i), nil
			}
			
			// Leading comments are captured as 'Comment' to associate them with the following block/field.
			if i < len(l.s) && l.s[i] == '#' {
				l.s = l.s[i:]
				l.pos.Column += i
				return l.consumeComment("Comment"), nil
			}
		}

		// Skip horizontal whitespace, updating column position accurately.
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

		// Handle newlines, resetting column and context-sensitive states.
		if l.s[0] == '\n' || l.s[0] == '\r' {
			l.consumeNewline()
			l.lastIdent = ""
			continue
		}
		break
	}

	// AttrValue state: greedily consume until comma, paren, or newline.
	// This captures raw expressions for after=, expect=, and iterate= attributes.
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

	// CodeValue state: captures nested JavaScript within code(...) or expressions.
	if l.expectingCode {
		l.expectingCode = false
		t := l.consumeBalanced('(', ')')
		t.Type = -12 // CodeValue
		return t, nil
	}

	var t lexer.Token
	var err error
	{
		r := l.s[0]
		switch {
		case r == '#':
			// Inline comments appear after other tokens on the same line.
			t = l.consumeComment("InlineComment")
		case r == '"':
			t = l.consumeString()
			l.lastIdent = ""
		case unicode.IsDigit(rune(r)) || r == '-' || r == '+':
			// Negative number check vs punctuation/direction indicator.
			if r == '-' {
				if strings.HasPrefix(l.s, "->") {
					t = l.consume(2, "Punct")
				} else if len(l.s) > 1 && unicode.IsDigit(rune(l.s[1])) {
					t = l.consumeNumber()
				} else {
					t = l.consume(1, "Punct")
				}
			} else {
				t = l.consumeNumber()
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
				l.lastIdent = ""
			} else {
				t = l.consume(1, "Punct")
				// Detect attribute assignment and code block start
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
			t = l.consume(1, "Punct")
			l.lastIdent = ""
		}
	}

	return t, err
}

// consumeBalanced captures text within balanced start/end runes, supporting nesting.
func (l *fragsLexer) consumeBalanced(start, end rune) lexer.Token {
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
	}
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
	case "Comment": return -2
	case "InlineComment": return -10
	case "String": return -3
	case "Number": return -4
	case "Bool": return -5
	case "Ident": return -6
	case "Punct": return -7
	case "Whitespace": return -8
	case "PromptItem": return -9
	case "AttrValue": return -11
	case "CodeValue": return -12
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

func (l *fragsLexer) consumeString() lexer.Token {
	i := 1
	for i < len(l.s) {
		if l.s[i] == '\\' && i+1 < len(l.s) {
			i += 2
			continue
		}
		if l.s[i] == '"' {
			i++
			break
		}
		i++
	}
	return l.consume(i, "String")
}

func (l *fragsLexer) consumeNumber() lexer.Token {
	i := 0
	if l.s[i] == '-' || l.s[i] == '+' {
		i++
	}
	for i < len(l.s) && (unicode.IsDigit(rune(l.s[i])) || l.s[i] == '.') {
		i++
	}
	return l.consume(i, "Number")
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

// consumePromptItem handles multi-line prompt blocks with indentation-based continuation rules.
func (l *fragsLexer) consumePromptItem(indent int) lexer.Token {
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
		
		// Continuation line must be indented strictly deeper than the '-' marker.
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
	token := lexer.Token{
		Type:  -9,
		Value: val,
		Pos:   l.pos,
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
	
	return token
}

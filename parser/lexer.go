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
		SymEOF:           TokenEOF,
		SymComment:       TokenComment,
		SymInlineComment: TokenInlineComment,
		SymString:        TokenString,
		SymRawString:     TokenRawString,
		SymNumber:        TokenNumber,
		SymBool:          TokenBool,
		SymIdent:         TokenIdent,
		SymPunct:         TokenPunct,
		SymWhitespace:    TokenWhitespace,
		SymPromptItem:    TokenPromptItem,
		SymPrePromptItem: TokenPrePromptItem,
		SymAttrValue:     TokenAttrValue,
		SymCodeValue:     TokenCodeValue,
	}
}

type fragsLexer struct {
	s                  string
	pos                lexer.Position
	lastIdent          string
	expectingAttrValue bool
	expectingCode      bool
	hasTokensOnLine    bool
}

func (l *fragsLexer) updatePos(val string) {
	for _, r := range val {
		if r == '\n' {
			l.pos.Line++
			l.pos.Column = 1
		} else {
			l.pos.Column++
		}
	}
}

func (l *fragsLexer) Next() (lexer.Token, error) {
	for {
		if len(l.s) == 0 {
			return lexer.Token{Type: TokenEOF, Pos: l.pos}, nil
		}

		if l.pos.Column == 1 {
			l.hasTokensOnLine = false
			i := 0
			for l.isSpace(i) {
				i++
			}
			isPrompt := l.isAtMarker('-', i)
			isPrePrompt := l.isAtMarker('+', i)

			if isPrompt || isPrePrompt {
				t, err := l.consumePromptItem(i, isPrePrompt)
				if err == nil {
					l.hasTokensOnLine = true
				}
				return t, err
			}
		}

		if l.isNewline(0) {
			l.consumeNewline()
			l.lastIdent = ""
			continue
		}

		if unicode.IsSpace(rune(l.s[0])) {
			l.consume(1, SymWhitespace)
			continue
		}
		break
	}

	if l.expectingAttrValue {
		l.expectingAttrValue = false
		if !l.isAt(0, '"') {
			i := 0
			for i < len(l.s) && l.s[i] != ',' && l.s[i] != ')' && !l.isNewline(i) {
				i++
			}
			val := strings.TrimSpace(l.s[:i])
			t := l.consume(i, SymAttrValue)
			t.Value = val
			l.hasTokensOnLine = true
			return t, nil
		}
	}

	if l.expectingCode {
		l.expectingCode = false
		t, err := l.consumeBalanced('(', ')')
		if err != nil {
			return t, err
		}
		t.Type = TokenCodeValue
		l.hasTokensOnLine = true
		return t, nil
	}

	var t lexer.Token
	var err error
	{
		r := l.s[0]
		switch {
		case r == '#':
			// Heuristic: If it starts at column 1 (after whitespace skipping), it's a block Comment.
			// Otherwise, it's an InlineComment.
			typ := SymInlineComment
			if !l.hasTokensOnLine {
				typ = SymComment
			}
			t = l.consumeComment(typ)
		case r == '"':
			t, err = l.consumeString()
			l.lastIdent = ""
		case r == '`':
			t, err = l.consumeRawString()
			l.lastIdent = ""
		case unicode.IsDigit(rune(r)) || r == '-' || r == '+':
			if r == '-' {
				if strings.HasPrefix(l.s, "->") {
					t = l.consume(2, SymPunct)
				} else if len(l.s) > 1 && unicode.IsDigit(rune(l.s[1])) {
					t, err = l.consumeNumber()
				} else {
					t = l.consume(1, SymPunct)
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
				t = l.consume(2, SymPunct)
				l.lastIdent = ""
			} else if strings.HasPrefix(l.s, "$(") {
				t = l.consume(2, SymPunct)
				l.expectingCode = true
				l.lastIdent = ""
			} else {
				t = l.consume(1, SymPunct)
				if t.Value == "=" {
					if l.isSessionAttribute(l.lastIdent) {
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
			return lexer.Token{}, l.lexerErrorf(l.pos, "illegal character %q", r)
		}
	}

	if err == nil && t.Type != TokenEOF {
		l.hasTokensOnLine = true
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
		return lexer.Token{}, l.lexerErrorf(startPos, "unclosed balanced block (missing %q)", end)
	}

	val := l.s[:i]
	l.s = l.s[i:]
	l.updatePos(val)

	return lexer.Token{
		Type:  TokenPunct,
		Value: val,
		Pos:   startPos,
	}, nil
}

func (l *fragsLexer) consumeNewline() {
	n := 1
	if strings.HasPrefix(l.s, "\r\n") {
		n = 2
	}
	val := l.s[:n]
	l.s = l.s[n:]
	l.updatePos(val)
}

func (l *fragsLexer) consume(n int, typ string) lexer.Token {
	val := l.s[:n]
	token := lexer.Token{
		Type:  l.typeToToken(typ),
		Value: val,
		Pos:   l.pos,
	}
	l.s = l.s[n:]
	l.updatePos(val)
	return token
}

func (l *fragsLexer) typeToToken(typ string) lexer.TokenType {
	switch typ {
	case SymComment:
		return TokenComment
	case SymInlineComment:
		return TokenInlineComment
	case SymString:
		return TokenString
	case SymNumber:
		return TokenNumber
	case SymBool:
		return TokenBool
	case SymIdent:
		return TokenIdent
	case SymPunct:
		return TokenPunct
	case SymWhitespace:
		return TokenWhitespace
	case SymPromptItem:
		return TokenPromptItem
	case SymPrePromptItem:
		return TokenPrePromptItem
	case SymAttrValue:
		return TokenAttrValue
	case SymCodeValue:
		return TokenCodeValue
	}
	return TokenEOF
}

func (l *fragsLexer) consumeComment(typ string) lexer.Token {
	i := 0
	for i < len(l.s) && !l.isNewline(i) {
		i++
	}
	return l.consume(i, typ)
}

func (l *fragsLexer) consumeString() (lexer.Token, error) {
	startPos := l.pos
	i := 1
	for i < len(l.s) {
		if l.isNewline(i) {
			return lexer.Token{}, l.lexerErrorf(startPos, "unterminated string literal")
		}
		if l.isAt(i, '\\') {
			if i+1 >= len(l.s) {
				return lexer.Token{}, l.lexerErrorf(startPos, "unterminated string literal")
			}
			esc := l.s[i+1]
			if esc != '"' && esc != '\\' && esc != 'n' && esc != 'r' && esc != 't' {
				return lexer.Token{}, l.lexerErrorf(l.pos, "invalid escape sequence \\%c", esc)
			}
			i += 2
			continue
		}
		if l.isAt(i, '"') {
			i++
			val := l.s[:i]
			// Validate template tags {{ }} are balanced
			if strings.Count(val, "{{") != strings.Count(val, "}}") {
				return lexer.Token{}, l.lexerErrorf(startPos, "malformed template tags in string")
			}
			token := lexer.Token{
				Type:  TokenString,
				Value: val,
				Pos:   startPos,
			}
			l.s = l.s[i:]
			l.updatePos(val)
			return token, nil
		}
		i++
	}
	return lexer.Token{}, l.lexerErrorf(startPos, "unterminated string literal")
}

func (l *fragsLexer) consumeRawString() (lexer.Token, error) {
	startPos := l.pos
	i := 1
	for i < len(l.s) {
		if l.isAt(i, '`') {
			i++
			val := l.s[:i]
			// Validate template tags
			if strings.Count(val, "{{") != strings.Count(val, "}}") {
				return lexer.Token{}, l.lexerErrorf(startPos, "malformed template tags in raw string")
			}
			token := lexer.Token{
				Type:  TokenRawString,
				Value: val,
				Pos:   startPos,
			}

			l.s = l.s[i:]
			l.updatePos(val)
			return token, nil
		}
		i++
	}
	return lexer.Token{}, l.lexerErrorf(startPos, "unterminated raw string literal")
}

func (l *fragsLexer) consumeIdent() lexer.Token {
	i := 0
	for l.isIdentChar(i) {
		i++
	}
	val := l.s[:i]
	if l.isBool(val) {
		return l.consume(i, SymBool)
	}
	return l.consume(i, SymIdent)
}

func (l *fragsLexer) consumeNumber() (lexer.Token, error) {
	startPos := l.pos
	i := 0
	if l.isAt(0, '-') || l.isAt(0, '+') {
		i++
	}

	hasDot := false
	for i < len(l.s) && (unicode.IsDigit(rune(l.s[i])) || l.s[i] == '.') {
		if l.s[i] == '.' {
			if hasDot {
				return lexer.Token{}, l.lexerErrorf(startPos, "multiple decimal points in number")
			}
			hasDot = true
		}
		i++
	}

	if hasDot && l.s[i-1] == '.' {
		return lexer.Token{}, l.lexerErrorf(startPos, "trailing decimal point in number")
	}

	return l.consume(i, SymNumber), nil
}

func (l *fragsLexer) consumePromptItem(indent int, isPrePrompt bool) (lexer.Token, error) {
	startPos := l.pos
	dashCol := indent + 1
	i := 0
	for i < len(l.s) && !l.isNewline(i) {
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
		} else if l.isNewline(j) {
			j++
		} else {
			break
		}

		k := 0
		for l.isSpace(j + k) {
			k++
		}

		if j+k < len(l.s) {
			if l.isNewline(j+k) || k > dashCol {
				// Either a blank line or an indented content line
				j += k
				for j < len(l.s) && !l.isNewline(j) {
					j++
				}
				totalLen = j
				continue
			}
		}
		break
	}

	val := l.s[:totalLen]
	// Validate template tags
	if strings.Count(val, "{{") != strings.Count(val, "}}") {
		return lexer.Token{}, l.lexerErrorf(startPos, "malformed template tags in prompt item")
	}

	typ := TokenPromptItem
	if isPrePrompt {
		typ = TokenPrePromptItem
	}

	token := lexer.Token{
		Type:  typ,
		Value: val,
		Pos:   startPos,
	}

	l.s = l.s[totalLen:]
	l.updatePos(val)

	return token, nil
}

func (l *fragsLexer) lexerErrorf(pos lexer.Position, format string, args ...interface{}) error {
	return &LexerError{
		Pos: pos,
		Msg: fmt.Sprintf(format, args...),
	}
}

package parser

import "unicode"

func (l *fragsLexer) peek(i int) byte {
	if i < 0 || i >= len(l.s) {
		return 0
	}
	return l.s[i]
}

func (l *fragsLexer) isAt(i int, c byte) bool {
	return i < len(l.s) && l.s[i] == c
}

func (l *fragsLexer) isSpace(i int) bool {
	c := l.peek(i)
	return c == ' ' || c == '\t'
}

func (l *fragsLexer) isNewline(i int) bool {
	c := l.peek(i)
	return c == '\n' || c == '\r'
}

func (l *fragsLexer) isAtMarker(marker byte, i int) bool {
	if !l.isAt(i, marker) {
		return false
	}
	next := i + 1
	return next == len(l.s) || l.isSpace(next) || l.isNewline(next)
}

func (l *fragsLexer) isIdentChar(i int) bool {
	c := l.peek(i)
	if c == 0 {
		return false
	}
	return unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) || c == '_' || c == '-'
}

func (l *fragsLexer) isBool(val string) bool {
	return val == "true" || val == "false"
}

func (l *fragsLexer) isSessionAttribute(ident string) bool {
	return ident == "expect" || ident == "iterate" || ident == "after"
}

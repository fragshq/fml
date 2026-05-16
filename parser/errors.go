package parser

import (
	"fmt"
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// LexerError represents an error that occurred during lexing, with position information.
type LexerError struct {
	Pos lexer.Position
	Msg string
}

func (e *LexerError) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

func (e *LexerError) Position() lexer.Position {
	return e.Pos
}

func (e *LexerError) Message() string {
	return e.Msg
}

// ErrorPosition extracts position information from an error if it implements
// the participle.Error interface or is a LexerError.
func ErrorPosition(err error) (pos lexer.Position, message string, ok bool) {
	if perr, ok := err.(participle.Error); ok {
		return perr.Position(), perr.Message(), true
	}
	if lerr, ok := err.(*LexerError); ok {
		return lerr.Pos, lerr.Msg, true
	}
	return lexer.Position{}, "", false
}

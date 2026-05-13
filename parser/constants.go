package parser

import "github.com/alecthomas/participle/v2/lexer"

const (
	TokenEOF           lexer.TokenType = -1
	TokenComment       lexer.TokenType = -2
	TokenString        lexer.TokenType = -3
	TokenNumber        lexer.TokenType = -4
	TokenBool          lexer.TokenType = -5
	TokenIdent         lexer.TokenType = -6
	TokenPunct         lexer.TokenType = -7
	TokenWhitespace    lexer.TokenType = -8
	TokenPromptItem    lexer.TokenType = -9
	TokenInlineComment lexer.TokenType = -10
	TokenAttrValue     lexer.TokenType = -11
	TokenCodeValue     lexer.TokenType = -12
	TokenPrePromptItem lexer.TokenType = -13
)

const (
	// Token Symbol Names
	SymComment       = "Comment"
	SymInlineComment = "InlineComment"
	SymString        = "String"
	SymNumber        = "Number"
	SymBool          = "Bool"
	SymIdent         = "Ident"
	SymPunct         = "Punct"
	SymWhitespace    = "Whitespace"
	SymPromptItem    = "PromptItem"
	SymPrePromptItem = "PrePromptItem"
	SymAttrValue     = "AttrValue"
	SymCodeValue     = "CodeValue"
	SymEOF           = "EOF"
)

const (
	// JSON Schema types
	TypeObject  = "object"
	TypeArray   = "array"
	TypeString  = "string"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeBoolean = "boolean"

	// DSL Scalar types
	ScalarString = "string"
	ScalarInt    = "int"
	ScalarFloat  = "float"
	ScalarBool   = "bool"
	ScalarAny    = "any"
)

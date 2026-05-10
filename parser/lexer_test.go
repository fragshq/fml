package parser

import (
	"strings"
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/stretchr/testify/assert"
)

func TestLexer_BasicTokens(t *testing.T) {
	def := &FRAGSLexerDefinition{}
	l, err := def.Lex("test.frags", strings.NewReader(`system("hello") set x = 5 true # comment`))
	assert.NoError(t, err)

	tokens, err := lexer.ConsumeAll(l)
	assert.NoError(t, err)

	expectedValues := []string{"system", "(", "\"hello\"", ")", "set", "x", "=", "5", "true", "# comment", ""}
	for i, tv := range expectedValues {
		assert.Equal(t, tv, tokens[i].Value)
	}
}

func TestLexer_PromptItem(t *testing.T) {
	def := &FRAGSLexerDefinition{}
	input := `- First line
  second line
- Item 2`
	l, err := def.Lex("test.frags", strings.NewReader(input))
	assert.NoError(t, err)

	tokens, err := lexer.ConsumeAll(l)
	assert.NoError(t, err)

	assert.Equal(t, -9, int(tokens[0].Type)) // PromptItem
	assert.Contains(t, tokens[0].Value, "First line")
	assert.Contains(t, tokens[0].Value, "second line")

	assert.Equal(t, -9, int(tokens[1].Type)) // PromptItem
	assert.Equal(t, "- Item 2", tokens[1].Value)
}

func TestLexer_Errors(t *testing.T) {
	def := &FRAGSLexerDefinition{}

	tests := []struct {
		name  string
		input string
		err   string
	}{
		{"UnterminatedString", `"no close`, "unterminated string literal"},
		{"InvalidEscape", `"bad \z"`, "invalid escape sequence"},
		{"MalformedNumber", `5.5.5`, "multiple decimal points"},
		{"TrailingDot", `5.`, "trailing decimal point"},
		{"IllegalChar", `set x = @`, "illegal character"},
		{"UnclosedCode", `code( x + 1`, "unclosed balanced block"},
		{"BrokenTemplate", `"{{ hi"`, "malformed template tags"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, _ := def.Lex("test.frags", strings.NewReader(tt.input))
			_, err := lexer.ConsumeAll(l)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestLexer_StateTransitions(t *testing.T) {
	def := &FRAGSLexerDefinition{}
	// Test AttrValue transition
	l, _ := def.Lex("test.frags", strings.NewReader(`session("s", expect=context.x > 0) { }`))
	tokens, err := lexer.ConsumeAll(l)
	assert.NoError(t, err)

	// Find AttrValue token
	found := false
	for _, tok := range tokens {
		if tok.Type == -11 { // AttrValue
			assert.Equal(t, "context.x > 0", tok.Value)
			found = true
		}
	}
	assert.True(t, found, "Should have produced an AttrValue token")
}

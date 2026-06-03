package lsp

import (
	"context"
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFMLHandler_Initialize(t *testing.T) {
	handler := &FMLHandler{}
	params := &lsp.InitializeParams{}

	resp, err := handler.Initialize(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "FML Language Server", resp.ServerInfo.Name)
	assert.True(t, *resp.Capabilities.HoverProvider)
	assert.NotNil(t, resp.Capabilities.CompletionProvider)
}

func TestFMLHandler_Diagnostics(t *testing.T) {
	handler := &FMLHandler{}
	handler.SetClient(nil) // We can't easily test PublishDiagnostics without a mock client, but we can check the diagnose logic if it were exposed or via DidOpen

	uri := lsp.DocumentURI("file:///test.fml")

	// Valid FML
	validFML := `system("test")
session("main") {
  - Hello
}`
	err := handler.DidOpen(context.Background(), &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  uri,
			Text: validFML,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, validFML, handler.documents[uri])

	// Invalid FML (syntax error)
	invalidFML := `system(`
	err = handler.DidOpen(context.Background(), &lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:  uri,
			Text: invalidFML,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, invalidFML, handler.documents[uri])
}

func TestFMLHandler_Completion(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "use \n",
	}

	// Test "use " completion
	params := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 0, Character: 4},
		},
	}

	resp, err := handler.Completion(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	foundMcp := false
	for _, item := range resp.Items {
		if item.Label == "mcp" {
			foundMcp = true
			break
		}
	}
	assert.True(t, foundMcp, "Should find 'mcp' completion after 'use '")
}

func TestFMLHandler_Completion_ParameterAttributes(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "parameter(\"p\", \n",
	}

	params := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 0, Character: 15}, // after "parameter("p", "
		},
	}

	resp, err := handler.Completion(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	foundEnum := false
	for _, item := range resp.Items {
		if item.Label == "enum=" {
			foundEnum = true
			break
		}
	}
	assert.True(t, foundEnum, "Should find 'enum=' completion for parameter")
}

func TestFMLHandler_Completion_ContextAwareness(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "# comment\n  + pre-prompt\n  - prompt\n",
	}

	// 1. Completion inside a comment
	paramsComment := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 0, Character: 5},
		},
	}
	respComment, err := handler.Completion(context.Background(), paramsComment)
	assert.NoError(t, err)
	assert.Empty(t, respComment.Items, "Should have no completions inside a comment")

	// 2. Completion inside a pre-prompt
	paramsPrePrompt := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 1, Character: 6},
		},
	}
	respPrePrompt, err := handler.Completion(context.Background(), paramsPrePrompt)
	assert.NoError(t, err)
	assert.Empty(t, respPrePrompt.Items, "Should have no completions inside a pre-prompt")

	// 2a. Completion after '+ ' (marker with space)
	handler.documents[uri] = "session(\"s\") {\n  + \n}"
	paramsPrePromptSpace := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 1, Character: 4}, // after "+ "
		},
	}
	respPrePromptSpace, err := handler.Completion(context.Background(), paramsPrePromptSpace)
	assert.NoError(t, err)
	assert.Empty(t, respPrePromptSpace.Items, "Should have no completions after '+ '")

	// 2b. Completion on blank line inside prompt
	handler.documents[uri] = "session(\"s\") {\n  - foo\n    \n}"
	paramsBlankInPrompt := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 2, Character: 4}, // on indented blank line
		},
	}
	respBlankInPrompt, err := handler.Completion(context.Background(), paramsBlankInPrompt)
	assert.NoError(t, err)
	assert.Empty(t, respBlankInPrompt.Items, "Should have no completions on blank line in prompt")

	// 3. Completion inside a prompt
	handler.documents[uri] = "# comment\n  + pre-prompt\n  - prompt\n"
	paramsPrompt := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 2, Character: 6},
		},
	}
	respPrompt, err := handler.Completion(context.Background(), paramsPrompt)
	assert.NoError(t, err)
	assert.Empty(t, respPrompt.Items, "Should have no completions inside a prompt")

	// 4. Completion on a continuation line
	handler.documents[uri] = "session(\"s\") {\n  + prompt\n    continuation\n}"
	paramsCont := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 2, Character: 8}, // inside "continuation"
		},
	}
	respCont, err := handler.Completion(context.Background(), paramsCont)
	assert.NoError(t, err)
	assert.Empty(t, respCont.Items, "Should have no completions on a prompt continuation line")
}

func TestFMLHandler_Completion_TransformerContext(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "transformer(\"t\") {\n  \n}",
	}

	params := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 1, Character: 2},
		},
	}

	resp, err := handler.Completion(context.Background(), params)
	assert.NoError(t, err)

	hasOnFunctionOutput := false
	hasAllowlist := false
	for _, item := range resp.Items {
		if item.Label == "onFunctionOutput =" {
			hasOnFunctionOutput = true
		}
		if item.Label == "allowlist =" {
			hasAllowlist = true
		}
	}

	assert.True(t, hasOnFunctionOutput, "Should suggest 'onFunctionOutput =' in transformer")
	assert.False(t, hasAllowlist, "Should NOT suggest 'allowlist =' in transformer")
}

func TestFMLHandler_Completion_SessionContext(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "session(\"s\") {\n  \n}",
	}

	params := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 1, Character: 2},
		},
	}

	resp, err := handler.Completion(context.Background(), params)
	assert.NoError(t, err)

	hasAllowlist := false
	for _, item := range resp.Items {
		if item.Label == "allowlist =" {
			hasAllowlist = true
		}
	}
	assert.False(t, hasAllowlist, "Should NOT suggest 'allowlist =' directly in session")
}

func TestFMLHandler_Completion_UseContext(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "session(\"s\") {\n  use mcp tool {\n    \n  }\n}",
	}

	params := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 2, Character: 4},
		},
	}

	resp, err := handler.Completion(context.Background(), params)
	assert.NoError(t, err)

	hasAllowlist := false
	for _, item := range resp.Items {
		if item.Label == "allowlist =" {
			hasAllowlist = true
		}
	}
	assert.True(t, hasAllowlist, "Should suggest 'allowlist =' inside use block")
}

func TestFMLHandler_Completion_ComponentsContext(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "components {\n  \n}",
	}

	params := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 1, Character: 2},
		},
	}

	resp, err := handler.Completion(context.Background(), params)
	assert.NoError(t, err)

	hasSchema := false
	hasPrompt := false
	for _, item := range resp.Items {
		if item.Label == "schema" {
			hasSchema = true
		}
		if item.Label == "prompt" {
			hasPrompt = true
		}
	}
	assert.True(t, hasSchema, "Should suggest 'schema' in components")
	assert.True(t, hasPrompt, "Should suggest 'prompt' in components")
}

func TestFMLHandler_Hover(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "system(\"test\")",
	}

	params := &lsp.HoverParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 0, Character: 2}, // inside "system"
		},
	}

	resp, err := handler.Hover(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Contents.Value, "Sets the global system prompt")

	// Test "enum" hover
	params.Position.Character = 0
	handler.documents[uri] = "enum=a|b"
	params.Position.Character = 2 // inside "enum"
	resp, err = handler.Hover(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Contents.Value, "Restricts a parameter or field")
}

func TestFMLHandler_SemanticTokens(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "system(\"test\")",
	}

	params := &lsp.SemanticTokensParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	}

	resp, err := handler.SemanticTokensFull(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Data)
}

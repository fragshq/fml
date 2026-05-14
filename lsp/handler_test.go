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

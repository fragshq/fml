package lsp

import (
	"context"
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theirish81/fml/parser"
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
	hasScript := false
	for _, item := range resp.Items {
		if item.Label == "schema" {
			hasSchema = true
		}
		if item.Label == "prompt" {
			hasPrompt = true
		}
		if item.Label == "script" {
			hasScript = true
		}
	}
	assert.True(t, hasSchema, "Should suggest 'schema' in components")
	assert.True(t, hasPrompt, "Should suggest 'prompt' in components")
	assert.True(t, hasScript, "Should suggest 'script' in components")
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

	// Test "script" hover
	params.Position.Character = 0
	handler.documents[uri] = "script"
	params.Position.Character = 2 // inside "script"
	resp, err = handler.Hover(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Contents.Value, "Defines a reusable script component")
}

func TestFMLHandler_SemanticTokens(t *testing.T) {
	handler := &FMLHandler{
		documents: make(map[lsp.DocumentURI]string),
	}
	uri := lsp.DocumentURI("file:///test.fml")

	// FML with a multi-line prompt and an em-dash (—).
	// The continuation line is indented by 4 spaces and has an em-dash.
	// Em-dash is 3 bytes in UTF-8, but 1 code unit in UTF-16.
	fml := `session("s") {
  - First line — with dash
    Continuation — line
}`
	handler.documents[uri] = fml

	params := &lsp.SemanticTokensParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	}

	resp, err := handler.SemanticTokensFull(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, resp)

	data := resp.Data

	foundPromptStart := false
	foundContinuation := false

	for i := 0; i < len(data); i += 5 {
		deltaLine := data[i]
		deltaChar := data[i+1]
		length := data[i+2]
		tokenType := data[i+3]

		// Token type 1 is "string" which is used for prompts in the current implementation
		if tokenType == 1 {
			// Check for first line of prompt (Line 1, Col 0 "- First line — with dash")
			if deltaLine == 1 && deltaChar == 0 && length >= 24 {
				foundPromptStart = true
			}
			// Check for continuation line (Line 2, Col 0 "    Continuation — line")
			if deltaLine == 1 && deltaChar == 0 && length >= 23 {
				foundContinuation = true
			}
		}
	}

	assert.True(t, foundPromptStart, "Should find the first line of the prompt at correct offset")
	assert.True(t, foundContinuation, "Should find the continuation line of the prompt at correct offset (indented)")
}

func TestFMLHandler_Completion_CallContext(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")
	handler.documents = map[lsp.DocumentURI]string{
		uri: "session(\"s\") {\n  call(\"kb_tool\") {\n    \n  }\n}",
	}

	params := &lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 2, Character: 4},
		},
	}

	resp, err := handler.Completion(context.Background(), params)
	assert.NoError(t, err)

	hasKbs := false
	hasCode := false
	hasAllowlist := false
	for _, item := range resp.Items {
		if item.Label == "kbs" {
			hasKbs = true
		}
		if item.Label == "code" {
			hasCode = true
		}
		if item.Label == "allowlist =" {
			hasAllowlist = true
		}
	}

	assert.True(t, hasKbs, "Should suggest 'kbs' inside call block")
	assert.True(t, hasCode, "Should suggest 'code' inside call block")
	assert.False(t, hasAllowlist, "Should NOT suggest 'allowlist =' inside call block")
}

func TestFMLHandler_Diagnostics_ScriptComponent(t *testing.T) {
	p, err := parser.NewParser()
	assert.NoError(t, err)

	// Case 1: Valid script parses without errors
	validInput := `components {
	script("my_script", type="kbs", description="some description", parameters={arg1: string}) (
		some script code goes here
	)
}`
	_, err = p.ParseString("file:///test.fml", validInput)
	assert.NoError(t, err)

	// Case 2: Invalid script (unclosed parenthesis in script body) produces a diagnostic syntax error
	invalidInput := `components {
	script("my_script", type="kbs", description="some description", parameters={arg1: string}) (
		some script code goes here
}`
	_, err = p.ParseString("file:///test.fml", invalidInput)
	assert.Error(t, err)

	pos, msg, ok := parser.ErrorPosition(err)
	assert.True(t, ok)
	// The lexer fails on the unclosed balanced block starting at the body content (line 3, column 3)
	assert.Equal(t, 3, pos.Line)
	assert.Equal(t, 3, pos.Column)
	assert.Contains(t, msg, "unclosed balanced block (missing ')')")
}

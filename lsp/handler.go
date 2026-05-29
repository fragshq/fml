package lsp

import (
	"context"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
	"github.com/theirish81/fml/parser"
)

type FMLHandler struct {
	client    *server.Client
	documents map[lsp.DocumentURI]string
}

func (h *FMLHandler) SetClient(client *server.Client) {
	h.client = client
	h.documents = make(map[lsp.DocumentURI]string)
}

func (h *FMLHandler) Initialize(ctx context.Context, params *lsp.InitializeParams) (*lsp.InitializeResult, error) {
	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptions{
				OpenClose: ptr(true),
				Change:    lsp.SyncFull,
			},
			CompletionProvider: &lsp.CompletionOptions{
				TriggerCharacters: []string{"(", ",", ":", "=", "$", " "},
			},
			HoverProvider:              ptr(true),
			DocumentFormattingProvider: ptr(true),
			DocumentOnTypeFormattingProvider: &lsp.DocumentOnTypeFormattingOptions{
				FirstTriggerCharacter: "\n",
			},
			SemanticTokensProvider: &lsp.SemanticTokensOptions{
				Legend: lsp.SemanticTokensLegend{
					TokenTypes: []string{
						"keyword", "string", "number", "comment", "type", "parameter", "variable",
					},
					TokenModifiers: []string{},
				},
				Full: &lsp.SemanticTokensFull{
					Delta: ptr(false),
				},
			},
		},
		ServerInfo: &lsp.ServerInfo{
			Name:    "FML Language Server",
			Version: "0.1.0",
		},
	}, nil
}

func (h *FMLHandler) Shutdown(ctx context.Context) error {
	return nil
}

func (h *FMLHandler) DidOpen(ctx context.Context, params *lsp.DidOpenTextDocumentParams) error {
	h.documents[params.TextDocument.URI] = params.TextDocument.Text
	h.diagnose(ctx, params.TextDocument.URI, params.TextDocument.Text)
	return nil
}

func (h *FMLHandler) DidChange(ctx context.Context, params *lsp.DidChangeTextDocumentParams) error {
	if len(params.ContentChanges) > 0 {
		text := params.ContentChanges[len(params.ContentChanges)-1].Text
		h.documents[params.TextDocument.URI] = text
		h.diagnose(ctx, params.TextDocument.URI, text)
	}
	return nil
}

func (h *FMLHandler) DidClose(ctx context.Context, params *lsp.DidCloseTextDocumentParams) error {
	delete(h.documents, params.TextDocument.URI)
	return nil
}

func (h *FMLHandler) OnTypeFormatting(ctx context.Context, params *lsp.DocumentOnTypeFormattingParams) ([]lsp.TextEdit, error) {
	if params.Character != "\n" {
		return nil, nil
	}

	text, ok := h.documents[params.TextDocument.URI]
	if !ok {
		return nil, nil
	}

	lines := strings.Split(text, "\n")
	if params.Position.Line <= 0 || params.Position.Line >= len(lines) {
		return nil, nil
	}

	prevLine := lines[params.Position.Line-1]
	trimmedPrev := strings.TrimSpace(prevLine)
	if trimmedPrev == "" {
		return nil, nil
	}

	// Calculate base indentation of previous line
	indent := ""
	for _, r := range prevLine {
		if r == ' ' || r == '\t' {
			indent += string(r)
		} else {
			break
		}
	}

	newIndent := indent
	// Rules for promoting indentation
	if strings.HasSuffix(trimmedPrev, "{") {
		// Basic block indentation
		tabSize := params.Options.TabSize
		if tabSize <= 0 {
			tabSize = 2
		}
		if params.Options.InsertSpaces {
			newIndent += strings.Repeat(" ", tabSize)
		} else {
			newIndent += "\t"
		}
	}

	return []lsp.TextEdit{
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: params.Position.Line, Character: 0},
				End:   lsp.Position{Line: params.Position.Line, Character: params.Position.Character},
			},
			NewText: newIndent,
		},
	}, nil
}

func (h *FMLHandler) diagnose(ctx context.Context, uri lsp.DocumentURI, text string) {
	diagnostics := make([]lsp.Diagnostic, 0)

	p, err := parser.NewParser()
	if err == nil {
		_, err = p.ParseString(string(uri), text)
		if err != nil {
			severity := lsp.SeverityError
			diag := lsp.Diagnostic{
				Severity: &severity,
				Message:  err.Error(),
				Range: lsp.Range{
					Start: lsp.Position{Line: 0, Character: 0},
					End:   lsp.Position{Line: 0, Character: 0},
				},
			}

			if pos, msg, ok := parser.ErrorPosition(err); ok {
				// LSP uses 0-based indexing
				lspPos := lsp.Position{Line: pos.Line - 1, Character: pos.Column - 1}
				diag.Range = lsp.Range{Start: lspPos, End: lspPos}
				diag.Message = msg
			}
			diagnostics = append(diagnostics, diag)
		}
	}

	if h.client != nil {
		_ = h.client.PublishDiagnostics(ctx, &lsp.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diagnostics,
		})
	}
}

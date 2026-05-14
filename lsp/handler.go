package main

import (
	"context"
	"fmt"
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
			HoverProvider: ptr(true),
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

func (h *FMLHandler) diagnose(ctx context.Context, uri lsp.DocumentURI, text string) {
	diagnostics := make([]lsp.Diagnostic, 0)

	p, err := parser.NewParser()
	if err == nil {
		_, err = p.ParseString(string(uri), text)
		if err != nil {
			msg := err.Error()
			severity := lsp.SeverityError
			diag := lsp.Diagnostic{
				Severity: &severity,
				Message:  msg,
				Range: lsp.Range{
					Start: lsp.Position{Line: 0, Character: 0},
					End:   lsp.Position{Line: 0, Character: 0},
				},
			}

			// Try to parse line and column from the error message
			// Format: <filename>:<line>:<col>: <message>
			parts := strings.SplitN(msg, ":", 4)
			if len(parts) >= 3 {
				var line, col int
				_, errLine := fmt.Sscanf(parts[1], "%d", &line)
				_, errCol := fmt.Sscanf(parts[2], "%d", &col)
				if errLine == nil && errCol == nil {
					// LSP uses 0-based indexing
					pos := lsp.Position{Line: line - 1, Character: col - 1}
					diag.Range = lsp.Range{Start: pos, End: pos}
					if len(parts) > 3 {
						diag.Message = strings.TrimSpace(parts[3])
					}
				}
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

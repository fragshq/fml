package lsp

import (
	"context"
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFMLHandler_OnTypeFormatting(t *testing.T) {
	handler := &FMLHandler{}
	uri := lsp.DocumentURI("file:///test.fml")

	tests := []struct {
		name     string
		content  string
		line     int
		char     int
		ch       string
		expected []lsp.TextEdit
	}{
		{
			name:    "indent after session open brace",
			content: "session(\"s\") {\n",
			line:    1,
			char:    0,
			ch:      "\n",
			expected: []lsp.TextEdit{
				{
					Range: lsp.Range{
						Start: lsp.Position{Line: 1, Character: 0},
						End:   lsp.Position{Line: 1, Character: 0},
					},
					NewText: "  ",
				},
			},
		},
		{
			name:    "no extra indent after prompt marker - (preserve level for new items)",
			content: "session(\"s\") {\n  -\n",
			line:    2,
			char:    0,
			ch:      "\n",
			expected: []lsp.TextEdit{
				{
					Range: lsp.Range{
						Start: lsp.Position{Line: 2, Character: 0},
						End:   lsp.Position{Line: 2, Character: 0},
					},
					NewText: "  ",
				},
			},
		},
		{
			name:    "handle existing auto-indent (replace it)",
			content: "session(\"s\") {\n    ",
			line:    1,
			char:    4,
			ch:      "\n",
			expected: []lsp.TextEdit{
				{
					Range: lsp.Range{
						Start: lsp.Position{Line: 1, Character: 0},
						End:   lsp.Position{Line: 1, Character: 4},
					},
					NewText: "  ",
				},
			},
		},
		{
			name:    "preserve indentation on regular lines",
			content: "session(\"s\") {\n  use search\n",
			line:    2,
			char:    0,
			ch:      "\n",
			expected: []lsp.TextEdit{
				{
					Range: lsp.Range{
						Start: lsp.Position{Line: 2, Character: 0},
						End:   lsp.Position{Line: 2, Character: 0},
					},
					NewText: "  ",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler.documents = map[lsp.DocumentURI]string{uri: tt.content}
			params := &lsp.DocumentOnTypeFormattingParams{
				TextDocumentPositionParams: lsp.TextDocumentPositionParams{
					TextDocument: lsp.TextDocumentIdentifier{URI: uri},
					Position:     lsp.Position{Line: tt.line, Character: tt.char},
				},
				Character: tt.ch,
				Options:   lsp.FormattingOptions{TabSize: 2, InsertSpaces: true},
			}

			edits, err := handler.OnTypeFormatting(context.Background(), params)
			if tt.expected == nil {
				assert.Error(t, err) // Or handle as no edits
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, edits)
			}
		})
	}
}

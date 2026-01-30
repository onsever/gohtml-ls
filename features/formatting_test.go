package features

import (
	"testing"

	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

func TestFormat_NormalizesWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		nEdits  int
	}{
		{
			name:   "extra spaces",
			input:  `{{  if   .X  }}hello{{  end  }}`,
			want:   `{{ if .X }}hello{{ end }}`,
			nEdits: 2,
		},
		{
			name:   "already normalized",
			input:  `{{ if .X }}hello{{ end }}`,
			want:   `{{ if .X }}hello{{ end }}`,
			nEdits: 0,
		},
		{
			name:   "trim markers preserved",
			input:  `{{-  range  .Items  -}}`,
			want:   `{{- range .Items -}}`,
			nEdits: 1,
		},
		{
			name:   "no spaces",
			input:  `{{.Field}}`,
			want:   `{{ .Field }}`,
			nEdits: 1,
		},
		{
			name:   "mixed",
			input:  `<p>{{  .Name  }}</p>{{.Age}}`,
			want:   `<p>{{ .Name }}</p>{{ .Age }}`,
			nEdits: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := tmpl.Parse("file:///test.gohtml", tt.input)
			edits := Format(pt)
			if len(edits) != tt.nEdits {
				t.Errorf("expected %d edits, got %d", tt.nEdits, len(edits))
				for _, e := range edits {
					t.Logf("  edit: %q", e.NewText)
				}
			}
			// Apply edits to verify final result
			if tt.nEdits > 0 {
				result := applyEdits(tt.input, pt, edits)
				if result != tt.want {
					t.Errorf("got %q, want %q", result, tt.want)
				}
			}
		})
	}
}

// applyEdits applies text edits in reverse order to avoid offset shifts.
func applyEdits(content string, pt *tmpl.ParsedTemplate, edits []lsp.TextEdit) string {
	// Apply in reverse order to maintain offsets
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		start := pt.PositionToOffset(e.Range.Start)
		end := pt.PositionToOffset(e.Range.End)
		content = content[:start] + e.NewText + content[end:]
	}
	return content
}

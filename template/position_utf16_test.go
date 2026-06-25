package template

import (
	"testing"

	"github.com/onsever/gohtml-ls/lsp"
)

func TestPositionConversion_UsesLSPUTF16Characters(t *testing.T) {
	pt := Parse("file:///test.html", "é{{.\n😀{{.")

	if pos := pt.OffsetToPosition(2); pos.Line != 0 || pos.Character != 1 {
		t.Fatalf("expected first brace after é at UTF-16 character 1, got line=%d character=%d", pos.Line, pos.Character)
	}
	if got := pt.PositionToOffset(lsp.Position{Line: 0, Character: 4}); got != len("é{{.") {
		t.Fatalf("expected UTF-16 character after dot on first line to map to byte offset %d, got %d", len("é{{."), got)
	}
	if pos := pt.OffsetToPosition(len("é{{.\n😀")); pos.Line != 1 || pos.Character != 2 {
		t.Fatalf("expected position after emoji at UTF-16 character 2, got line=%d character=%d", pos.Line, pos.Character)
	}
	if got := pt.PositionToOffset(lsp.Position{Line: 1, Character: 5}); got != len("é{{.\n😀{{.") {
		t.Fatalf("expected UTF-16 character after dot on second line to map to byte offset %d, got %d", len("é{{.\n😀{{."), got)
	}
}

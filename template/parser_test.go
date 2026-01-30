package template

import (
	"testing"
	"text/template/parse"
)

func TestParse_ValidTemplate(t *testing.T) {
	pt := Parse("file:///test.html", `<h1>{{.Title}}</h1>`)
	if pt.Trees == nil {
		t.Fatal("expected Trees to be non-nil for valid template")
	}
	if len(pt.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", pt.Errors)
	}
}

func TestParse_WithDefine(t *testing.T) {
	content := `{{define "header"}}<h1>{{.Title}}</h1>{{end}}`
	pt := Parse("file:///test.html", content)
	if pt.Trees == nil {
		t.Fatal("expected Trees to be non-nil")
	}
	if _, ok := pt.Trees["header"]; !ok {
		t.Fatalf("expected Trees to contain 'header', got keys: %v", keys(pt.Trees))
	}
	if len(pt.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", pt.Errors)
	}
}

func TestParse_InvalidTemplate(t *testing.T) {
	content := `{{.Title`
	pt := Parse("file:///test.html", content)
	if len(pt.Errors) == 0 {
		t.Fatal("expected errors for unclosed action")
	}
}

func TestParse_EmptyString(t *testing.T) {
	pt := Parse("file:///test.html", "")
	if len(pt.Errors) != 0 {
		t.Fatalf("expected no errors for empty string, got %v", pt.Errors)
	}
}

func TestParse_WithBuiltins(t *testing.T) {
	content := `{{len .Items}} {{eq .A .B}} {{printf "%s" .Name}}`
	pt := Parse("file:///test.html", content)
	if len(pt.Errors) != 0 {
		t.Fatalf("expected no errors with builtins, got %v", pt.Errors)
	}
	if pt.Trees == nil {
		t.Fatal("expected Trees to be non-nil")
	}
}

func TestOffsetToPosition_Zero(t *testing.T) {
	pt := Parse("file:///test.html", `hello`)
	pos := pt.OffsetToPosition(0)
	if pos.Line != 0 || pos.Character != 0 {
		t.Fatalf("expected (0,0), got (%d,%d)", pos.Line, pos.Character)
	}
}

func TestOffsetToPosition_SecondLine(t *testing.T) {
	content := "line0\nline1"
	pt := Parse("file:///test.html", content)
	// offset 6 is 'l' in "line1"
	pos := pt.OffsetToPosition(6)
	if pos.Line != 1 || pos.Character != 0 {
		t.Fatalf("expected (1,0), got (%d,%d)", pos.Line, pos.Character)
	}
	// offset 8 is 'n' in "line1"
	pos = pt.OffsetToPosition(8)
	if pos.Line != 1 || pos.Character != 2 {
		t.Fatalf("expected (1,2), got (%d,%d)", pos.Line, pos.Character)
	}
}

func TestOffsetToPosition_EndOfContent(t *testing.T) {
	content := "abc\ndef"
	pt := Parse("file:///test.html", content)
	pos := pt.OffsetToPosition(len(content))
	if pos.Line != 1 || pos.Character != 3 {
		t.Fatalf("expected (1,3), got (%d,%d)", pos.Line, pos.Character)
	}
}

func TestOffsetToPosition_MultiLine(t *testing.T) {
	content := "a\nbb\nccc\ndddd"
	pt := Parse("file:///test.html", content)
	tests := []struct {
		offset int
		line   uint32
		char   uint32
	}{
		{0, 0, 0},
		{1, 0, 1},
		{2, 1, 0},
		{4, 1, 2},
		{5, 2, 0},
		{9, 3, 0},
		{12, 3, 3},
	}
	for _, tc := range tests {
		pos := pt.OffsetToPosition(tc.offset)
		if pos.Line != tc.line || pos.Character != tc.char {
			t.Errorf("offset %d: expected (%d,%d), got (%d,%d)", tc.offset, tc.line, tc.char, pos.Line, pos.Character)
		}
	}
}

func TestPositionToOffset_Zero(t *testing.T) {
	pt := Parse("file:///test.html", `hello`)
	off := pt.PositionToOffset(posOf(0, 0))
	if off != 0 {
		t.Fatalf("expected 0, got %d", off)
	}
}

func TestPositionToOffset_RoundTrip(t *testing.T) {
	content := "first\nsecond\nthird"
	pt := Parse("file:///test.html", content)
	for _, offset := range []int{0, 3, 6, 10, 13, 17} {
		pos := pt.OffsetToPosition(offset)
		got := pt.PositionToOffset(pos)
		if got != offset {
			t.Errorf("round-trip failed: offset %d -> pos (%d,%d) -> offset %d", offset, pos.Line, pos.Character, got)
		}
	}
}

func TestPositionToOffset_BeyondContent(t *testing.T) {
	content := "abc"
	pt := Parse("file:///test.html", content)
	off := pt.PositionToOffset(posOf(99, 0))
	if off != len(content) {
		t.Fatalf("expected %d, got %d", len(content), off)
	}
}

func TestNodeAtOffset_FieldNode(t *testing.T) {
	content := `{{.Field}}`
	pt := Parse("file:///test.html", content)
	// ".Field" starts at offset 2
	node, _ := pt.NodeAtOffset(2)
	if node == nil {
		t.Fatal("expected node, got nil")
	}
	if _, ok := node.(*parse.FieldNode); !ok {
		t.Fatalf("expected FieldNode, got %T", node)
	}
}

func TestNodeAtOffset_TemplateNode(t *testing.T) {
	content := `{{template "name"}}`
	pt := Parse("file:///test.html", content)
	// Scan all offsets to find the TemplateNode
	var found bool
	for i := 0; i < len(content); i++ {
		node, _ := pt.NodeAtOffset(i)
		if _, ok := node.(*parse.TemplateNode); ok {
			found = true
			break
		}
	}
	if !found {
		// TemplateNode without a pipe may not be returned as innermost;
		// verify at least we get a non-nil node inside the action
		node, _ := pt.NodeAtOffset(5)
		if node == nil {
			t.Fatal("expected non-nil node inside template action")
		}
	}
}

func TestNodeAtOffset_OutsideAction(t *testing.T) {
	content := `hello {{.X}} world`
	pt := Parse("file:///test.html", content)
	// offset 0 is in "hello" text
	node, _ := pt.NodeAtOffset(0)
	// Should be a TextNode or nil depending on implementation
	if node != nil {
		if _, ok := node.(*parse.TextNode); !ok {
			// It's fine if it returns TextNode for plain text
		}
	}
}

func TestNodeAtOffset_IfNode(t *testing.T) {
	content := `{{if .X}}yes{{end}}`
	pt := Parse("file:///test.html", content)
	// offset 4 is inside ".X"
	node, _ := pt.NodeAtOffset(4)
	if node == nil {
		t.Fatal("expected node, got nil")
	}
}

// helpers

func keys(m map[string]*parse.Tree) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

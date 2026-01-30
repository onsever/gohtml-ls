package template

import (
	"testing"
)

func TestNewTemplateIndex(t *testing.T) {
	idx := NewTemplateIndex()
	if idx == nil {
		t.Fatal("expected non-nil index")
	}
	if idx.Definitions == nil || idx.References == nil || idx.Trees == nil {
		t.Fatal("expected non-nil maps")
	}
	if len(idx.Definitions) != 0 || len(idx.References) != 0 {
		t.Fatal("expected empty maps")
	}
}

func TestIndex_Define(t *testing.T) {
	idx := NewTemplateIndex()
	content := `{{define "header"}}<h1>Header</h1>{{end}}`
	pt := Parse("file:///a.html", content)
	idx.Index(pt)

	locs, ok := idx.Definitions["header"]
	if !ok || len(locs) == 0 {
		t.Fatal("expected definition for 'header'")
	}
	if locs[0].URI != "file:///a.html" {
		t.Fatalf("expected URI file:///a.html, got %s", locs[0].URI)
	}
}

func TestIndex_TemplateCall(t *testing.T) {
	idx := NewTemplateIndex()
	content := `{{template "header"}}`
	pt := Parse("file:///b.html", content)
	idx.Index(pt)

	locs, ok := idx.References["header"]
	if !ok || len(locs) == 0 {
		t.Fatal("expected reference for 'header'")
	}
	if locs[0].URI != "file:///b.html" {
		t.Fatalf("expected URI file:///b.html, got %s", locs[0].URI)
	}
}

func TestIndex_DefineAndTemplateCall(t *testing.T) {
	idx := NewTemplateIndex()

	def := `{{define "nav"}}<nav/>{{end}}`
	ptDef := Parse("file:///nav.html", def)
	idx.Index(ptDef)

	ref := `{{template "nav"}}`
	ptRef := Parse("file:///page.html", ref)
	idx.Index(ptRef)

	if _, ok := idx.Definitions["nav"]; !ok {
		t.Fatal("expected definition for 'nav'")
	}
	if _, ok := idx.References["nav"]; !ok {
		t.Fatal("expected reference for 'nav'")
	}
}

func TestIndex_ReindexReplaces(t *testing.T) {
	idx := NewTemplateIndex()

	content1 := `{{define "old"}}<p>old</p>{{end}}`
	pt1 := Parse("file:///a.html", content1)
	idx.Index(pt1)

	if _, ok := idx.Definitions["old"]; !ok {
		t.Fatal("expected definition for 'old' after first index")
	}

	content2 := `{{define "new"}}<p>new</p>{{end}}`
	pt2 := Parse("file:///a.html", content2)
	idx.Index(pt2)

	if _, ok := idx.Definitions["old"]; ok {
		t.Fatal("expected 'old' definition to be removed after re-index")
	}
	if _, ok := idx.Definitions["new"]; !ok {
		t.Fatal("expected definition for 'new' after re-index")
	}
}

func TestIndex_MultipleURIs(t *testing.T) {
	idx := NewTemplateIndex()

	pt1 := Parse("file:///a.html", `{{define "a"}}<a/>{{end}}`)
	pt2 := Parse("file:///b.html", `{{define "b"}}<b/>{{end}}`)
	idx.Index(pt1)
	idx.Index(pt2)

	if _, ok := idx.Definitions["a"]; !ok {
		t.Fatal("expected definition for 'a'")
	}
	if _, ok := idx.Definitions["b"]; !ok {
		t.Fatal("expected definition for 'b'")
	}
}

func TestIndex_RemoveURI(t *testing.T) {
	idx := NewTemplateIndex()

	pt := Parse("file:///a.html", `{{define "x"}}<x/>{{end}}{{template "y"}}`)
	idx.Index(pt)

	if _, ok := idx.Definitions["x"]; !ok {
		t.Fatal("expected definition for 'x'")
	}
	if _, ok := idx.References["y"]; !ok {
		t.Fatal("expected reference for 'y'")
	}

	// Re-index with empty content to simulate removal
	ptEmpty := Parse("file:///a.html", "")
	idx.Index(ptEmpty)

	if _, ok := idx.Definitions["x"]; ok {
		t.Fatal("expected definition for 'x' to be removed")
	}
	if _, ok := idx.References["y"]; ok {
		t.Fatal("expected reference for 'y' to be removed")
	}
}

func TestIndex_Block(t *testing.T) {
	idx := NewTemplateIndex()
	content := `{{block "sidebar" .}}<div>default</div>{{end}}`
	pt := Parse("file:///layout.html", content)
	idx.Index(pt)

	if _, ok := idx.Definitions["sidebar"]; !ok {
		t.Fatal("expected definition for 'sidebar' from block")
	}
}

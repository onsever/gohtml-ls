package lsp

import "testing"

func TestDocumentStore_OpenAndGet(t *testing.T) {
	ds := NewDocumentStore()
	ds.Open("file:///a.gohtml", 1, "hello")
	doc := ds.Get("file:///a.gohtml")
	if doc == nil {
		t.Fatal("expected document")
	}
	if doc.Content != "hello" {
		t.Errorf("expected content 'hello', got %q", doc.Content)
	}
}

func TestDocumentStore_Change(t *testing.T) {
	ds := NewDocumentStore()
	ds.Open("file:///a.gohtml", 1, "hello")
	ds.Change("file:///a.gohtml", 2, "world")
	doc := ds.Get("file:///a.gohtml")
	if doc.Content != "world" {
		t.Errorf("expected content 'world', got %q", doc.Content)
	}
}

func TestDocumentStore_Close(t *testing.T) {
	ds := NewDocumentStore()
	ds.Open("file:///a.gohtml", 1, "hello")
	ds.Close("file:///a.gohtml")
	if ds.Get("file:///a.gohtml") != nil {
		t.Error("expected nil after close")
	}
}

func TestDocumentStore_GetNonExistent(t *testing.T) {
	ds := NewDocumentStore()
	if ds.Get("file:///nope") != nil {
		t.Error("expected nil for non-existent document")
	}
}

func TestDocumentStore_Multiple(t *testing.T) {
	ds := NewDocumentStore()
	ds.Open("file:///a.gohtml", 1, "aaa")
	ds.Open("file:///b.gohtml", 1, "bbb")
	if ds.Get("file:///a.gohtml").Content != "aaa" {
		t.Error("wrong content for a")
	}
	if ds.Get("file:///b.gohtml").Content != "bbb" {
		t.Error("wrong content for b")
	}
}

package lsp

import "sync"

// DocumentStore tracks open text documents.
type DocumentStore struct {
	docs map[string]*Document
	mu   sync.RWMutex
}

// Document represents an open text document.
type Document struct {
	URI     string
	Version int
	Content string
}

func NewDocumentStore() *DocumentStore {
	return &DocumentStore{docs: make(map[string]*Document)}
}

func (ds *DocumentStore) Open(uri string, version int, content string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.docs[uri] = &Document{URI: uri, Version: version, Content: content}
}

func (ds *DocumentStore) Change(uri string, version int, content string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if doc, ok := ds.docs[uri]; ok {
		doc.Version = version
		doc.Content = content
	} else {
		ds.docs[uri] = &Document{URI: uri, Version: version, Content: content}
	}
}

func (ds *DocumentStore) Close(uri string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.docs, uri)
}

func (ds *DocumentStore) Get(uri string) *Document {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.docs[uri]
}

// IsOpen returns true if the URI is currently open in the editor.
func (ds *DocumentStore) IsOpen(uri string) bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	_, ok := ds.docs[uri]
	return ok
}

// All returns all open documents.
func (ds *DocumentStore) All() []*Document {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	result := make([]*Document, 0, len(ds.docs))
	for _, doc := range ds.docs {
		result = append(result, doc)
	}
	return result
}

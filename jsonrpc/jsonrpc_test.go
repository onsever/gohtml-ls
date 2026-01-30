package jsonrpc

import (
	"encoding/json"
	"testing"
)

func TestNewResponse(t *testing.T) {
	id := json.RawMessage(`1`)
	resp, err := NewResponse(id, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	if string(resp.ID) != "1" {
		t.Errorf("expected id 1, got %s", string(resp.ID))
	}
	var m map[string]string
	if err := json.Unmarshal(resp.Result, &m); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %s", m["key"])
	}
}

func TestNewResponseNilResult(t *testing.T) {
	id := json.RawMessage(`1`)
	resp, err := NewResponse(id, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Result) != "null" {
		t.Errorf("expected null result, got %s", string(resp.Result))
	}
}

func TestNewErrorResponse(t *testing.T) {
	id := json.RawMessage(`2`)
	resp := NewErrorResponse(id, MethodNotFound, "method not found")
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	if resp.Error == nil {
		t.Fatal("expected error to be set")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("expected code %d, got %d", MethodNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "method not found" {
		t.Errorf("expected message 'method not found', got %s", resp.Error.Message)
	}
}

func TestNewNotification(t *testing.T) {
	params := map[string]int{"line": 10}
	n, err := NewNotification("textDocument/didOpen", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", n.JSONRPC)
	}
	if n.Method != "textDocument/didOpen" {
		t.Errorf("unexpected method: %s", n.Method)
	}
	var p map[string]int
	if err := json.Unmarshal(n.Params, &p); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	if p["line"] != 10 {
		t.Errorf("expected line=10, got %d", p["line"])
	}
}

func TestResponseErrorError(t *testing.T) {
	e := &ResponseError{Code: InternalError, Message: "something broke"}
	if e.Error() != "something broke" {
		t.Errorf("expected 'something broke', got %s", e.Error())
	}
}

func TestRequestJSONRoundTrip(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`42`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"cap":true}`),
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.Method != req.Method {
		t.Errorf("method mismatch: %s vs %s", got.Method, req.Method)
	}
	if string(got.ID) != string(req.ID) {
		t.Errorf("id mismatch")
	}
}

func TestResponseJSONRoundTrip(t *testing.T) {
	resp, _ := NewResponse(json.RawMessage(`1`), "ok")
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var got Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if string(got.Result) != string(resp.Result) {
		t.Errorf("result mismatch: %s vs %s", string(got.Result), string(resp.Result))
	}
}

func TestNotificationJSONRoundTrip(t *testing.T) {
	n, _ := NewNotification("exit", nil)
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var got Notification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.Method != "exit" {
		t.Errorf("method mismatch: %s", got.Method)
	}
}

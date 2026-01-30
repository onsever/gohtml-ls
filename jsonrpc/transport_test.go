package jsonrpc

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestReadMessage(t *testing.T) {
	input := "Content-Length: 13\r\n\r\n{\"id\":\"test\"}"
	tr := NewTransport(strings.NewReader(input), nil)
	msg, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg) != `{"id":"test"}` {
		t.Errorf("unexpected message: %s", string(msg))
	}
}

func TestReadMessageLFOnly(t *testing.T) {
	input := "Content-Length: 13\n\n{\"id\":\"test\"}"
	tr := NewTransport(strings.NewReader(input), nil)
	msg, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg) != `{"id":"test"}` {
		t.Errorf("unexpected message: %s", string(msg))
	}
}

func TestReadMessageMissingContentLength(t *testing.T) {
	input := "X-Custom: foo\r\n\r\n{}"
	tr := NewTransport(strings.NewReader(input), nil)
	_, err := tr.ReadMessage()
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}

func TestReadMessageMultiple(t *testing.T) {
	msg1 := `{"a":1}`
	msg2 := `{"b":2}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msg1), msg1) +
		fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msg2), msg2)
	tr := NewTransport(strings.NewReader(input), nil)

	got1, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("read 1: %v", err)
	}
	if string(got1) != msg1 {
		t.Errorf("msg1 mismatch: %s", string(got1))
	}

	got2, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("read 2: %v", err)
	}
	if string(got2) != msg2 {
		t.Errorf("msg2 mismatch: %s", string(got2))
	}
}

func TestWriteMessage(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(nil, &buf)
	body := []byte(`{"result":true}`)
	if err := tr.WriteMessage(body); err != nil {
		t.Fatalf("write error: %v", err)
	}
	expected := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriteMessageThreadSafety(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(nil, &buf)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			msg := fmt.Sprintf(`{"i":%d}`, n)
			if err := tr.WriteMessage([]byte(msg)); err != nil {
				t.Errorf("write error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	// Verify we can parse all 50 messages back
	reader := NewTransport(strings.NewReader(buf.String()), nil)
	for i := 0; i < 50; i++ {
		_, err := reader.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message %d: %v", i, err)
		}
	}
}

func TestWriteThenRead(t *testing.T) {
	var buf bytes.Buffer
	writer := NewTransport(nil, &buf)
	body := []byte(`{"jsonrpc":"2.0","id":1}`)
	if err := writer.WriteMessage(body); err != nil {
		t.Fatalf("write error: %v", err)
	}
	reader := NewTransport(strings.NewReader(buf.String()), nil)
	msg, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(msg) != string(body) {
		t.Errorf("round-trip mismatch: %s vs %s", string(msg), string(body))
	}
}

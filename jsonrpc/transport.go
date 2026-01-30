package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

type Transport struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
}

func NewTransport(in io.Reader, out io.Writer) *Transport {
	return &Transport{
		reader: bufio.NewReader(in),
		writer: out,
	}
}

func (t *Transport) ReadMessage() (json.RawMessage, error) {
	contentLength := -1

	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			val := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %s", val)
			}
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(t.reader, body); err != nil {
		return nil, err
	}

	return json.RawMessage(body), nil
}

func (t *Transport) WriteMessage(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(t.writer, header); err != nil {
		return err
	}
	_, err := t.writer.Write(data)
	return err
}

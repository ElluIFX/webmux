package session

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

func TestAlreadyCancelledInputIsNotWritten(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	s := &Session{terminal: writer, inputGate: make(chan struct{}, 1), done: make(chan struct{})}
	s.inputGate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The gate and cancellation are simultaneously ready. Exercise both select
	// outcomes without depending on terminal echo or child-process scheduling.
	for range 100 {
		if err := s.WriteContext(ctx, []byte("x")); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled input returned %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	written, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Fatalf("cancelled input delivered %d bytes", len(written))
	}
}

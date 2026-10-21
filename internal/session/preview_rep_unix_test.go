//go:build linux || darwin

package session

import (
	"context"
	"testing"
	"time"
)

func TestShutdownDuringRepeatedPreviewControls(t *testing.T) {
	t.Parallel()
	m := NewManager(Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	s, err := m.Create("", "custom", []string{"/bin/sh", "-c",
		`while :; do printf '\033[100000Smarker\n'; sleep 0.01; done`,
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	// The preview's count clamp must never rewrite the original PTY stream.
	waitForOutput(t, s, "\x1b[100000Smarker")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		m.Shutdown(ctx)
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("preview output prevented bounded shutdown")
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("session cleanup did not finish")
	}
}

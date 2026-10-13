package session

import (
	"bytes"
	"testing"
)

func TestRingWrap(t *testing.T) {
	r := NewRingBuffer(5)
	for _, step := range []struct{ in, want string }{{"ab", "ab"}, {"cde", "abcde"}, {"fg", "cdefg"}, {"1234567", "34567"}, {"8", "45678"}, {"", "45678"}} {
		r.Write([]byte(step.in))
		if got := string(r.Snapshot()); got != step.want {
			t.Fatalf("got %q want %q", got, step.want)
		}
	}
	snap := r.Snapshot()
	snap[0] = 'x'
	if bytes.Equal(snap, r.Snapshot()) {
		t.Fatal("snapshot aliases storage")
	}
}
func TestRingBinary(t *testing.T) {
	r := NewRingBuffer(4)
	r.Write([]byte{0, 27, 255})
	r.Write([]byte{1, 2})
	if !bytes.Equal(r.Snapshot(), []byte{27, 255, 1, 2}) {
		t.Fatal(r.Snapshot())
	}
}

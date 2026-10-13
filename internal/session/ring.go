package session

import "sync"

// RingBuffer retains the newest capacity bytes without interpreting terminal output.
type RingBuffer struct {
	mu         sync.RWMutex
	data       []byte
	next, size int
}

func NewRingBuffer(capacity int) *RingBuffer {
	if capacity < 1 {
		panic("ring capacity must be positive")
	}
	return &RingBuffer{data: make([]byte, capacity)}
}
func (r *RingBuffer) Write(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(p) >= len(r.data) {
		p = p[len(p)-len(r.data):]
		copy(r.data, p)
		r.next = 0
		r.size = len(p)
		return
	}
	n := copy(r.data[r.next:], p)
	copy(r.data, p[n:])
	r.next = (r.next + len(p)) % len(r.data)
	r.size = min(len(r.data), r.size+len(p))
}
func (r *RingBuffer) Snapshot() []byte {
	return r.snapshotWithPrefix(nil)
}

// snapshotWithPrefix assembles the protocol header and history in one allocation.
func (r *RingBuffer) snapshotWithPrefix(prefix []byte) []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]byte, len(prefix)+r.size)
	copy(out, prefix)
	start := (r.next - r.size + len(r.data)) % len(r.data)
	n := copy(out[len(prefix):], r.data[start:])
	copy(out[len(prefix)+n:], r.data[:r.next])
	return out
}

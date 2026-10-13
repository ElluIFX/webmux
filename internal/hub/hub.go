package hub

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/session"
	"time"
)

func write(ctx context.Context, c *websocket.Conn, kind websocket.MessageType, p []byte) error {
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.Write(deadline, kind, p)
}

// Attach owns the only data writer. Its reader is cancelled and joined on all exits.
func Attach(ctx context.Context, s *session.Session, c *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	sub := s.Attach()
	defer s.Detach(sub)
	c.SetReadLimit(1024 * 1024)
	readerDone := make(chan struct{})
	inputDone := make(chan struct{})
	input := make(chan []byte, 16)
	go func() {
		defer close(inputDone)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case p := <-input:
				if s.WriteContext(ctx, p) != nil {
					return
				}
			}
		}
	}()
	go func() {
		defer close(readerDone)
		defer cancel()
		for {
			kind, p, err := c.Read(ctx)
			if err != nil {
				return
			}
			if kind != websocket.MessageBinary || len(p) == 0 {
				continue
			}
			switch p[0] {
			case 0:
				select {
				case input <- p[1:]:
				case <-ctx.Done():
					return
				default:
					cancel()
					return
				}
			case 1:
				var size struct {
					Cols uint16 `json:"cols"`
					Rows uint16 `json:"rows"`
				}
				if json.Unmarshal(p[1:], &size) != nil || s.Resize(size.Cols, size.Rows) != nil {
					continue
				}
			case 2:
				s.Replay(sub)
			}
		}
	}()
	defer func() { cancel(); _ = c.CloseNow(); <-readerDone; <-inputDone }()
	var pending []byte
	for {
		var frame []byte
		if pending != nil {
			frame = pending
			pending = nil
		} else {
			select {
			case <-ctx.Done():
				return
			case <-sub.Slow:
				_ = c.Close(4408, "slow client")
				return
			case frame = <-sub.Out:
			}
		}
		if frame[0] == 0 {
			combined := make([]byte, len(frame), 65537)
			copy(combined, frame)
			timer := time.NewTimer(50 * time.Millisecond)
		aggregate:
			for len(combined) < 65537 {
				select {
				case next := <-sub.Out:
					if next[0] != 0 || len(combined)+len(next)-1 > 65537 {
						pending = next
						break aggregate
					}
					combined = append(combined, next[1:]...)
				case <-timer.C:
					break aggregate
				case <-ctx.Done():
					timer.Stop()
					return
				case <-sub.Slow:
					timer.Stop()
					_ = c.Close(4408, "slow client")
					return
				}
			}
			timer.Stop()
			frame = combined
		}
		if write(ctx, c, websocket.MessageBinary, frame) != nil {
			_ = c.Close(4408, "write timeout")
			return
		}
		if frame[0] == 3 {
			_ = c.Close(4410, "session exited")
			return
		}
	}
}
func List(ctx context.Context, m *session.Manager, c *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	sub := m.Subscribe()
	defer m.Unsubscribe(sub)
	reader := c.CloseRead(ctx)
	defer func() { cancel(); _ = c.CloseNow() }()
	for {
		select {
		case <-reader.Done():
			return
		case <-sub.Slow:
			_ = c.Close(4408, "slow client")
			return
		case event := <-sub.Out:
			data, err := json.Marshal(event)
			if err != nil || write(ctx, c, websocket.MessageText, data) != nil {
				_ = c.Close(4408, "write timeout")
				return
			}
		}
	}
}

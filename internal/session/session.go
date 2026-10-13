package session

import (
	"encoding/json"
	"errors"
	"github.com/creack/pty"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusWaiting Status = "waiting"
	StatusIdle    Status = "idle"
	StatusExited  Status = "exited"
)

type Info struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Cwd          string    `json:"cwd"`
	Preset       string    `json:"preset"`
	Cmd          []string  `json:"cmd"`
	Status       Status    `json:"status"`
	ExitCode     *int      `json:"exitCode"`
	LastLine     string    `json:"lastLine"`
	CreatedAt    time.Time `json:"createdAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
	Cols         uint16    `json:"cols"`
	Rows         uint16    `json:"rows"`
	Clients      int       `json:"clients"`
}
type Subscriber struct {
	Out    chan []byte
	Slow   chan struct{}
	failed bool
}
type Session struct {
	mu         sync.Mutex
	inputMu    sync.Mutex
	info       Info
	userTitle  string
	terminal   *os.File
	cmd        *exec.Cmd
	ring       *RingBuffer
	osc        OSCScanner
	preview    Preview
	subs       map[*Subscriber]struct{}
	done       chan struct{}
	readerDone chan struct{}
	closeOnce  sync.Once
	notify     func()
}

func start(id, cwd, preset, name string, argv []string, capacity int, notify func()) (*Session, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "WEBMUX_SESSION_ID="+id)
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	s := &Session{info: Info{ID: id, Title: filepath.Base(cwd), Cwd: cwd, Preset: preset, Cmd: append([]string(nil), argv...), Status: StatusRunning, CreatedAt: now, LastActiveAt: now, Cols: 80, Rows: 24}, userTitle: name, terminal: f, cmd: cmd, ring: NewRingBuffer(capacity), subs: make(map[*Subscriber]struct{}), done: make(chan struct{}), readerDone: make(chan struct{}), notify: notify}
	if name != "" {
		s.info.Title = name
	}
	go s.read()
	go s.wait()
	return s, nil
}
func (s *Session) Info() Info { s.mu.Lock(); defer s.mu.Unlock(); return s.infoLocked() }
func (s *Session) infoLocked() Info {
	i := s.info
	i.Clients = len(s.subs)
	i.Cmd = append([]string(nil), i.Cmd...)
	if i.ExitCode != nil {
		code := *i.ExitCode
		i.ExitCode = &code
	}
	return i
}
func (s *Session) Buffer() []byte        { return s.ring.Snapshot() }
func (s *Session) Done() <-chan struct{} { return s.done }
func Frame(kind byte, p []byte) []byte {
	frame := make([]byte, len(p)+1)
	frame[0] = kind
	copy(frame[1:], p)
	return frame
}
func (s *Session) stateLocked() []byte {
	data, _ := json.Marshal(s.infoLocked())
	return Frame(2, data)
}
func (s *Session) sendLocked(sub *Subscriber, p []byte) {
	if sub.failed {
		return
	}
	select {
	case sub.Out <- p:
	default:
		sub.failed = true
		close(sub.Slow)
	}
}
func (s *Session) broadcastLocked(p []byte) {
	for sub := range s.subs {
		s.sendLocked(sub, p)
	}
}

// Attach atomically captures history and registers the live subscriber: no replay gap.
func (s *Session) Attach() *Subscriber {
	s.mu.Lock()
	sub := &Subscriber{Out: make(chan []byte, 256), Slow: make(chan struct{})}
	s.subs[sub] = struct{}{}
	s.replayLocked(sub)
	s.mu.Unlock()
	s.notify()
	return sub
}
func (s *Session) replayLocked(sub *Subscriber) {
	s.sendLocked(sub, Frame(1, s.ring.Snapshot()))
	s.sendLocked(sub, s.stateLocked())
	if s.info.ExitCode != nil {
		data, _ := json.Marshal(map[string]int{"exitCode": *s.info.ExitCode})
		s.sendLocked(sub, Frame(3, data))
	}
}
func (s *Session) Replay(sub *Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		select {
		case <-sub.Out:
		default:
			s.replayLocked(sub)
			return
		}
	}
}
func (s *Session) Detach(sub *Subscriber) {
	s.mu.Lock()
	delete(s.subs, sub)
	s.mu.Unlock()
	s.notify()
}
func (s *Session) Write(p []byte) error {
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	select {
	case <-s.done:
		return errors.New("session exited")
	default:
	}
	_, err := s.terminal.Write(p)
	return err
}
func (s *Session) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 || cols > 1000 || rows > 1000 {
		return errors.New("invalid terminal size")
	}
	s.mu.Lock()
	err := pty.Setsize(s.terminal, &pty.Winsize{Cols: cols, Rows: rows})
	if err == nil {
		s.info.Cols = cols
		s.info.Rows = rows
	}
	s.mu.Unlock()
	s.notify()
	return err
}
func (s *Session) read() {
	defer close(s.readerDone)
	buf := make([]byte, 32768)
	for {
		n, err := s.terminal.Read(buf)
		if n > 0 {
			p := buf[:n]
			s.mu.Lock()
			s.ring.Write(p)
			s.osc.Feed(p, func(title string) {
				if s.userTitle == "" {
					if title == "" {
						title = filepath.Base(s.info.Cwd)
					}
					s.info.Title = title
				}
			})
			s.info.LastLine = s.preview.Feed(p)
			s.info.LastActiveAt = time.Now()
			s.info.Status = StatusRunning
			s.broadcastLocked(Frame(0, p))
			s.mu.Unlock()
			s.notify()
		}
		if err != nil {
			return
		}
	}
}
func (s *Session) wait() {
	err := s.cmd.Wait()
	code := 0
	if err != nil {
		code = s.cmd.ProcessState.ExitCode()
		if code < 0 {
			if status, ok := s.cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
				code = 128 + int(status.Signal())
			}
		}
	} // Drain final PTY bytes before publishing exit.
	_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGHUP)
	select {
	case <-s.readerDone:
	case <-time.After(time.Second):
		_ = s.terminal.Close()
		<-s.readerDone
	}
	_ = s.terminal.Close()
	s.mu.Lock()
	s.info.Status = StatusExited
	s.info.ExitCode = &code
	s.broadcastLocked(s.stateLocked())
	data, _ := json.Marshal(map[string]int{"exitCode": code})
	s.broadcastLocked(Frame(3, data))
	close(s.done)
	s.mu.Unlock()
	s.notify()
	slog.Info("session exited", "id", s.info.ID, "exitCode", code)
}
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		exited := s.info.ExitCode != nil
		s.mu.Unlock()
		if exited {
			return
		}
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGHUP)
		go func() {
			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
			case <-s.done:
			}
		}()
	})
}

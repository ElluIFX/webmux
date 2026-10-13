package session

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Preset struct {
	Name      string   `json:"name" toml:"name"`
	Label     string   `json:"label" toml:"label"`
	Cmd       []string `json:"cmd" toml:"cmd"`
	AllowArgs bool     `json:"allowArgs" toml:"allow_args"`
}
type Options struct {
	ScrollbackBytes int
	DefaultCwd      string
	Presets         []Preset
}
type Event struct {
	Type     string `json:"type"`
	Sessions []Info `json:"sessions,omitempty"`
	Session  *Info  `json:"session,omitempty"`
	ID       string `json:"id,omitempty"`
}
type ListSubscriber struct {
	Out    chan Event
	Slow   chan struct{}
	failed bool
}
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	subs     map[*ListSubscriber]struct{}
	options  Options
	dirty    chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	stopped  bool
}

var ErrNotFound = errors.New("session not found")
var ErrRunning = errors.New("session is still running")

func NewManager(o Options) *Manager {
	if o.ScrollbackBytes == 0 {
		o.ScrollbackBytes = 2 * 1024 * 1024
	}
	m := &Manager{sessions: make(map[string]*Session), subs: make(map[*ListSubscriber]struct{}), options: o, dirty: make(chan struct{}, 1), done: make(chan struct{})}
	go m.updates()
	return m
}
func ExpandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return filepath.Abs(path)
}
func (m *Manager) Create(cwd, preset string, args []string, name string) (*Session, error) {
	if cwd == "" {
		cwd = m.options.DefaultCwd
	}
	cwd, err := ExpandPath(cwd)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(cwd)
	if err != nil {
		return nil, fmt.Errorf("directory: %w", err)
	}
	if !stat.IsDir() {
		return nil, errors.New("path is not a directory")
	}
	var argv []string
	for _, p := range m.options.Presets {
		if p.Name == preset {
			if len(args) > 0 && !p.AllowArgs {
				return nil, errors.New("preset does not allow arguments")
			}
			argv = append(append([]string(nil), p.Cmd...), args...)
			break
		}
	}
	if len(argv) == 0 {
		return nil, errors.New("unknown or empty preset")
	}
	var raw [5]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return nil, err
	}
	id := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:]))
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return nil, errors.New("server is shutting down")
	}
	s, err := start(id, cwd, preset, name, argv, m.options.ScrollbackBytes, m.markDirty)
	if err != nil {
		return nil, err
	}
	m.sessions[id] = s
	info := s.Info()
	m.publishLocked(Event{Type: "created", Session: &info})
	slog.Info("session created", "id", id, "preset", preset)
	return s, nil
}
func (m *Manager) markDirty() {
	select {
	case m.dirty <- struct{}{}:
	default:
	}
}
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}
func (m *Manager) List() []Info { m.mu.Lock(); defer m.mu.Unlock(); return m.listLocked() }
func (m *Manager) listLocked() []Info {
	list := make([]Info, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s.Info())
	}
	sort.Slice(list, func(i, j int) bool { return list[i].LastActiveAt.After(list[j].LastActiveAt) })
	return list
}
func (m *Manager) Close(id string) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	s.Close()
	return nil
}
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return ErrNotFound
	}
	if s.Info().Status != StatusExited {
		return ErrRunning
	}
	delete(m.sessions, id)
	m.publishLocked(Event{Type: "removed", ID: id})
	return nil
}
func (m *Manager) Subscribe() *ListSubscriber {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub := &ListSubscriber{Out: make(chan Event, 256), Slow: make(chan struct{})}
	sub.Out <- Event{Type: "snapshot", Sessions: m.listLocked()}
	m.subs[sub] = struct{}{}
	return sub
}
func (m *Manager) Unsubscribe(sub *ListSubscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subs, sub)
}
func (m *Manager) publishLocked(e Event) {
	for sub := range m.subs {
		if sub.failed {
			continue
		}
		select {
		case sub.Out <- e:
		default:
			sub.failed = true
			close(sub.Slow)
		}
	}
}
func (m *Manager) updates() {
	timer := time.NewTicker(500 * time.Millisecond)
	defer timer.Stop()
	dirty := false
	for {
		select {
		case <-m.done:
			return
		case <-m.dirty:
			dirty = true
		case <-timer.C:
			if dirty {
				m.mu.Lock()
				for _, s := range m.sessions {
					info := s.Info()
					m.publishLocked(Event{Type: "updated", Session: &info})
				}
				m.mu.Unlock()
				dirty = false
			}
		}
	}
}
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	m.stopped = true
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	m.mu.Unlock()
	for _, s := range list {
		s.Close()
	}
	for _, s := range list {
		select {
		case <-s.Done():
		case <-ctx.Done():
			_ = syscallKill(s)
		}
	}
	m.stopOnce.Do(func() { close(m.done) })
}
func syscallKill(s *Session) error { return killGroup(s.cmd.Process.Pid) }

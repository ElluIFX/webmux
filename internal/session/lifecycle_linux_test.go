//go:build linux

package session

import (
	"bytes"
	"context"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLeaderExitWithInheritedPTY(t *testing.T) {
	m := NewManager(Options{DefaultCwd: t.TempDir(), Presets: []Preset{{Name: "child", Cmd: []string{"/bin/bash", "-c", "trap '' HUP; sleep 30 & printf 'CHILD_STARTED\\n'; exit 4"}}}})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	s, err := m.Create("", "child", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-time.After(7 * time.Second):
		t.Fatal("leader exited but retained slave PTY prevented final exit")
	}
	if info := s.Info(); info.ExitCode == nil || *info.ExitCode != 4 {
		t.Fatalf("exit info: %+v", info)
	}
	assertNoSessionProcesses(t, s)
}

func assertNoSessionProcesses(t *testing.T, s *Session) {
	t.Helper()
	for pid, state := range processes() {
		if state.sid == s.cmd.Process.Pid {
			t.Fatalf("session descendant still exists: pid %d state %c", pid, state.state)
		}
	}
}

func TestCloseReapsSeparateJobGroups(t *testing.T) {
	m := NewManager(Options{DefaultCwd: t.TempDir(), Presets: []Preset{{Name: "job", Cmd: []string{"/bin/bash", "-c", `set -m; (trap '' HUP; printf 'CHILD_READY\n'; exec sleep 30) & wait`}}}})
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	defer m.Shutdown(ctx)
	s, err := m.Create("", "job", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(string(s.Buffer()), "CHILD_READY") {
		if time.Now().After(deadline) {
			t.Fatal("child never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.Close()
	select {
	case <-s.Done():
	case <-ctx.Done():
		t.Fatal("close failed to terminate child job")
	}
	assertNoSessionProcesses(t, s)
}

func TestCancelledPTYWriteDoesNotHoldInputGate(t *testing.T) {
	m := NewManager(Options{DefaultCwd: t.TempDir(), Presets: []Preset{{Name: "blocked", Cmd: []string{"/bin/bash", "-c", `stty raw -echo; printf READY; sleep 30`}}}})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	s, err := m.Create("", "blocked", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(string(s.Buffer()), "READY") {
		if time.Now().After(deadline) {
			t.Fatal("terminal never entered raw mode")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err = s.Resize(100, 40); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(100*time.Millisecond, cancel)
	defer timer.Stop()
	start := time.Now()
	err = s.WriteContext(ctx, bytes.Repeat([]byte("x"), 512*1024))
	if !errors.Is(err, context.Canceled) || time.Since(start) > 2*time.Second {
		t.Fatalf("uncancelled write: %v", err)
	}
	next, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stop()
	start = time.Now()
	_ = s.WriteContext(next, []byte("y"))
	if time.Since(start) > time.Second {
		t.Fatal("cancelled writer retained serialization gate")
	}
}

func TestAdoptedZombieReapedWhileLeaderLives(t *testing.T) {
	m := NewManager(Options{DefaultCwd: t.TempDir(), Presets: []Preset{{Name: "daemon", Cmd: []string{"/bin/bash", "-c", `setsid /bin/bash -c 'sleep 0.05 & printf "DAEMON_CHILD=%s\n" "$!"'; read -r stop; exit 23`}}}})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	s, err := m.Create("", "daemon", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`DAEMON_CHILD=(\d+)`)
	var match []string
	until := time.Now().Add(time.Second)
	for len(match) == 0 {
		match = pattern.FindStringSubmatch(string(s.Buffer()))
		if time.Now().After(until) {
			t.Fatal("daemon child was not announced")
		}
		time.Sleep(10 * time.Millisecond)
	}
	child, _ := strconv.Atoi(match[1])
	defer func() { var status syscall.WaitStatus; _, _ = syscall.Wait4(child, &status, syscall.WNOHANG, nil) }()
	until = time.Now().Add(3 * time.Second)
	for {
		_, err = os.Stat("/proc/" + strconv.Itoa(child))
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(until) {
			t.Fatal("adopted new-session zombie remains while manager is alive")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if s.Info().Status == StatusExited {
		t.Fatal("reaper consumed the live session leader")
	}
	if err = s.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("leader wait did not finish")
	}
	if info := s.Info(); info.ExitCode == nil || *info.ExitCode != 23 {
		t.Fatalf("leader wait result stolen: %+v", info)
	}
}

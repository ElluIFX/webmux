//go:build windows

package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0magnet/xterm-go/vt"
	"golang.org/x/sys/windows"
)

// A native test executable avoids depending on Git Bash or PowerShell profiles.
func TestWindowsTerminalHelper(t *testing.T) {
	if os.Getenv("WEBMUX_SESSION_ID") == "" {
		return
	}
	args := os.Args
	switch args[len(args)-1] {
	case "flood":
		for {
			fmt.Print(strings.Repeat("output ", 4096))
		}
	case "hold":
		time.Sleep(time.Minute)
	case "tree":
		child := exec.Command(os.Args[0], "-test.run=^TestWindowsTerminalHelper$", "--", "hold")
		if err := child.Start(); err != nil {
			os.Exit(90)
		}
		fmt.Printf("CHILD_PID=%d\r\n", child.Process.Pid)
		// Wait until the test has opened a process handle before the leader exits.
		var line string
		fmt.Fscanln(os.Stdin, &line)
		fmt.Print("FINAL_MARKER\r\n")
		os.Exit(7)
	case "echo":
		fmt.Print("READY_MARKER\r\n")
		var line string
		fmt.Fscanln(os.Stdin, &line)
		fmt.Printf("REPLY_%s\r\n", line)
		os.Exit(23)
	}
	os.Exit(0)
}

func windowsTestSession(t *testing.T, mode string) *Session {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return newIdleSession(t, executable, "-test.run=^TestWindowsTerminalHelper$", "--", mode)
}
func waitWindowsOutput(t *testing.T, s *Session, marker string) string {
	t.Helper()
	until := time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		text := string(s.Buffer())
		if strings.Contains(text, marker) {
			return text
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("terminal did not emit %q: %q", marker, s.Buffer())
	return ""
}
func waitWindowsExit(t *testing.T, s *Session, code int) {
	t.Helper()
	select {
	case <-s.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("terminal exit did not finish")
	}
	if info := s.Info(); info.ExitCode == nil || *info.ExitCode != code {
		t.Fatalf("exit: %+v", info)
	}
}
func TestWindowsConPTYInputResizeReplayAndExit(t *testing.T) {
	s := windowsTestSession(t, "echo")
	waitWindowsOutput(t, s, "READY_MARKER")
	if err := s.Resize(110, 33); err != nil {
		t.Fatal(err)
	}
	if err := s.Write([]byte("hello\r")); err != nil {
		t.Fatal(err)
	}
	waitWindowsExit(t, s, 23)
	if !bytes.Contains(s.Buffer(), []byte("REPLY_hello")) {
		t.Fatalf("final output missing: %q", s.Buffer())
	}
	sub := s.Attach()
	defer s.Detach(sub)
	frames := queuedKinds(sub)
	if len(frames) < 4 || frames[len(frames)-1] != 3 {
		t.Fatalf("replay exit ordering: %v", frames)
	}
}
func TestWindowsLeaderExitKillsDescendants(t *testing.T) {
	s := windowsTestSession(t, "tree")
	text := waitWindowsOutput(t, s, "CHILD_PID=")
	// ConPTY can split the marker and its decimal value across reads.
	var pid int
	until := time.Now().Add(5 * time.Second)
	for pid == 0 && time.Now().Before(until) {
		text = string(s.Buffer())
		start := strings.Index(text, "CHILD_PID=") + len("CHILD_PID=")
		end := strings.IndexAny(text[start:], "\r\n")
		if end >= 0 {
			pid, _ = strconv.Atoi(text[start : start+end])
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatalf("child PID missing: %q", text)
	}
	child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(child)
	if err = s.Write([]byte("exit\r")); err != nil {
		t.Fatal(err)
	}
	waitWindowsExit(t, s, 7)
	status, err := windows.WaitForSingleObject(child, 5000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("descendant survived leader exit: status=%d, err=%v", status, err)
	}
	if !bytes.Contains(s.Buffer(), []byte("FINAL_MARKER")) {
		t.Fatalf("final output not drained: %q", s.Buffer())
	}
}
func TestWindowsCloseUnblocksReader(t *testing.T) {
	s := windowsTestSession(t, "hold")
	s.Close()
	select {
	case <-s.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("close blocked")
	}
	select {
	case <-s.readerDone:
	default:
		t.Fatal("reader leaked")
	}
}
func TestWindowsInputCancellationPreservesSession(t *testing.T) {
	s := windowsTestSession(t, "hold")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.WriteContext(ctx, []byte("not delivered")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled input: %v", err)
	}
	select {
	case <-s.Done():
		t.Fatal("cancellation killed the persistent session")
	default:
	}
	if err := s.Resize(90, 30); err != nil {
		t.Fatal(err)
	}
}
func TestWindowsBlockedPipeWriteCancellation(t *testing.T) {
	writer, reader, err := newWindowsInputPipe()
	if err != nil {
		t.Fatal(err)
	}
	input := &windowsTerminal{input: writer, changed: make(chan struct{})}
	defer input.Close()
	output := os.NewFile(uintptr(reader), "terminal-test-input")
	defer output.Close()
	s := &Session{terminal: input, inputGate: make(chan struct{}, 1), done: make(chan struct{})}
	s.inputGate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := s.WriteContext(ctx, bytes.Repeat([]byte("x"), 1<<20)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked write: %v", err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("input cancellation took too long")
	}
	drained := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, output); close(drained) }()
	if err := s.WriteContext(context.Background(), []byte("after cancellation")); err != nil {
		t.Fatalf("pipe unusable after cancellation: %v", err)
	}
	input.Close()
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		t.Fatal("reader did not stop")
	}
}

func TestWindowsEnvironmentCaseAndUnicode(t *testing.T) {
	env, err := windowsEnvironment([]string{"Path=first", "PATH=second", "UNICODE=中文", "=C:=C:\\work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(env) < 2 || env[len(env)-1] != 0 || env[len(env)-2] != 0 {
		t.Fatal("environment is not double-NUL terminated")
	}
	if _, err = windowsEnvironment([]string{"BAD=\x00"}); err == nil {
		t.Fatal("NUL accepted")
	}
	sanitized := sessionEnv([]string{"term=bad", "wt_session=host", "Path=ok"}, "test", false)
	for _, entry := range sanitized {
		if entry == "term=bad" || entry == "wt_session=host" {
			t.Fatalf("host environment leaked: %q", entry)
		}
	}
}

func TestWindowsStartPreservesCommandLookupError(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	cmd.Err = exec.ErrDot
	if _, err := startTerminal(cmd, "lookup-test"); !errors.Is(err, exec.ErrDot) {
		t.Fatalf("command lookup error bypassed: %v", err)
	}
}

func TestWindowsResizeConcurrentWithCloseDrainsOutput(t *testing.T) {
	s := windowsTestSession(t, "flood")
	waitWindowsOutput(t, s, "output")
	resized := make(chan struct{})
	go func() {
		defer close(resized)
		for i := 0; i < 1000; i++ {
			if err := s.Resize(uint16(80+i%20), 24); err != nil {
				return
			}
		}
	}()
	s.Close()
	select {
	case <-s.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("resize/close deadlocked with output reader")
	}
	select {
	case <-resized:
	case <-time.After(time.Second):
		t.Fatal("resize did not unblock")
	}
}

func TestWindowsBatchLaunchersRequireExplicitExecutable(t *testing.T) {
	for _, path := range []string{"cli.cmd", "cli.BAT"} {
		cmd := &exec.Cmd{Path: path, Args: []string{path, "literal & argument"}}
		if _, err := startTerminal(cmd, "batch-test"); err == nil || !strings.Contains(err.Error(), "node.exe") {
			t.Fatalf("batch launcher %q: %v", path, err)
		}
	}
}

func TestWindowsExecutableUsesSessionDirectoryAndPathExt(t *testing.T) {
	dir := t.TempDir()
	image := filepath.Join(dir, "session-tool.exe")
	if err := os.WriteFile(image, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATHEXT", ".EXE;.CMD")
	for _, name := range []string{`.\session-tool.exe`, `.\session-tool`} {
		got, err := windowsExecutable(name, dir)
		if err != nil || !strings.EqualFold(got, image) {
			t.Fatalf("resolve %q: %q, %v; want %q", name, got, err, image)
		}
	}
	if _, err := windowsExecutable(`C:session-tool.exe`, dir); err == nil {
		t.Fatal("ambiguous drive-relative image accepted")
	}
}

func TestWindowsExpandPathAcceptsNativeHomeSeparator(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	got, err := ExpandPath(`~\projects`)
	if err != nil || got != filepath.Join(home, "projects") {
		t.Fatalf("native home path: %q, %v", got, err)
	}
}

func TestWindowsExpiredWriteDeadlineDoesNotDeliverBytes(t *testing.T) {
	writer, reader, err := newWindowsInputPipe()
	if err != nil {
		t.Fatal(err)
	}
	input := &windowsTerminal{input: writer, changed: make(chan struct{})}
	defer input.Close()
	output := os.NewFile(uintptr(reader), "expired-terminal-test-input")
	defer output.Close()
	if err := input.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := input.Write([]byte("must not be delivered")); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expired write: %d, %v", n, err)
	}
	input.Close()
	data, err := io.ReadAll(output)
	if err != nil || len(data) != 0 {
		t.Fatalf("expired write delivered %q: %v", data, err)
	}
}

// Run a second test process with redirected stdio, as happens under go test
// and service/logging hosts. The session's output must stay in its ConPTY,
// not appear in the hosting process's redirected stdout/stderr.
func TestWindowsConPTYWithRedirectedParent(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestWindowsRedirectedParentHelper$")
	cmd.Env = append(os.Environ(), "WEBMUX_TEST_REDIRECTED_PARENT=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("redirected parent failed: %v\n%s", err, output)
	}
	if bytes.Contains(output, []byte("CHILD_CAPTURE_ONLY")) {
		t.Fatalf("session output leaked to server stdio: %q", output)
	}
	if !bytes.Contains(output, []byte("PARENT_CAPTURE_OK")) {
		t.Fatalf("helper did not verify capture: %q", output)
	}
}

func TestWindowsRedirectedParentHelper(t *testing.T) {
	if os.Getenv("WEBMUX_TEST_REDIRECTED_PARENT") != "1" {
		return
	}
	s := newIdleSession(t, "cmd.exe", "/d", "/c", "echo CHILD_CAPTURE_ONLY")
	waitWindowsExit(t, s, 0)
	if !bytes.Contains(s.Buffer(), []byte("CHILD_CAPTURE_ONLY")) {
		t.Fatalf("ConPTY lost child output: %q", s.Buffer())
	}
	fmt.Println("PARENT_CAPTURE_OK")
}

// Exercise the actual default shell, not only a Go console helper: PSReadLine
// consumes console key records differently from a cooked stdin Read call.
func TestWindowsPowerShellInteractiveInput(t *testing.T) {
	shell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	s := newIdleSession(t, shell, "-NoLogo", "-NoExit")
	attachWindowsTestPeer(t, s)
	waitWindowsOutput(t, s, "> ")
	// Separate writes reproduce the browser's per-key packet boundaries.
	for _, key := range []byte("Write-Output ('NATIVE_' + 'POWERSHELL_READY')\r") {
		if err := s.Write([]byte{key}); err != nil {
			t.Fatal(err)
		}
	}
	waitWindowsOutput(t, s, "NATIVE_POWERSHELL_READY")
	if err := s.Write([]byte("exit 29\r")); err != nil {
		t.Fatal(err)
	}
	waitWindowsExit(t, s, 29)
}

// This peer belongs only to the integration test. Production Preview must
// never answer device queries: the controlling browser owns those replies.
// Use the existing VT parser so split CSI requests and cursor coordinates
// are handled correctly rather than inventing a fixed CPR for PowerShell.
func attachWindowsTestPeer(t *testing.T, s *Session) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	sub := s.Attach()
	options := vt.NewOptions()
	options.Cols, options.Rows, options.Scrollback = 80, 24, 0
	terminal := vt.NewTerminal(options)
	terminal.OnData = func(reply string) {
		if err := s.WriteInput(ctx, []byte(reply), true); err != nil && ctx.Err() == nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("test terminal reply: %v", err)
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-sub.Slow:
				t.Error("test terminal peer fell behind")
				return
			case frame := <-sub.Out:
				if len(frame) == 0 {
					continue
				}
				switch frame[0] {
				case 0, 1:
					terminal.Write(frame[1:])
				case 3:
					return
				}
			}
		}
	}()
	t.Cleanup(func() { cancel(); <-done; s.Detach(sub) })
}

func TestWindowsTestPeerAnswersSplitDeviceQueries(t *testing.T) {
	options := vt.NewOptions()
	options.Cols, options.Rows = 80, 24
	terminal := vt.NewTerminal(options)
	var replies []string
	terminal.OnData = func(reply string) { replies = append(replies, reply) }
	for _, part := range []string{"\x1b[2;4H", "\x1b[", "6n", "\x1b[5", "n"} {
		terminal.Write([]byte(part))
	}
	if len(replies) != 2 || replies[0] != "\x1b[2;4R" || replies[1] != "\x1b[0n" {
		t.Fatalf("device replies: %q", replies)
	}
}

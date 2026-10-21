//go:build windows

package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ConPTY is available on Windows 10 1809 / Windows Server 2019 and later.
// The child is created suspended and assigned to a kill-on-close job before
// it can execute, so even immediately spawned grandchildren stay contained.
type windowsTerminal struct {
	output      *os.File
	input       windows.Handle
	console     windows.Handle
	process     windows.Handle
	job         windows.Handle
	pid         int
	consoleMu   sync.Mutex
	consoleOnce sync.Once
	closeOnce   sync.Once
	writeMu     sync.Mutex
	mu          sync.Mutex
	deadline    time.Time
	changed     chan struct{}
	closed      bool
}

var windowsTerminals sync.Map // PID -> *windowsTerminal, removed after Wait

func startTerminal(cmd *exec.Cmd, id string) (_ terminalIO, err error) {
	if err := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreatePseudoConsole").Find(); err != nil {
		return nil, fmt.Errorf("ConPTY requires Windows 10 1809 or newer: %w", err)
	}
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	resolved, err := windowsExecutable(cmd.Path, cmd.Dir)
	if err != nil {
		return nil, err
	}
	resolved, argv, err := windowsLauncher(resolved, cmd.Args)
	if err != nil {
		return nil, err
	}
	t := &windowsTerminal{changed: make(chan struct{})}
	defer func() {
		if err != nil {
			t.Close()
		}
	}()
	// The service end is overlapped, permitting cancellation of a full input
	// pipe without closing the persistent terminal on a browser disconnect.
	var childInput windows.Handle
	t.input, childInput, err = newWindowsInputPipe()
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(childInput)
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	t.output = outputRead
	defer outputWrite.Close()
	if err = windows.CreatePseudoConsole(windows.Coord{X: 80, Y: 24}, childInput, windows.Handle(outputWrite.Fd()), 0, &t.console); err != nil {
		return nil, fmt.Errorf("create ConPTY: %w", err)
	}
	t.job, err = windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(t.job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, err
	}
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attrs.Delete()
	// PSEUDOCONSOLE takes the handle value, not a pointer to the handle.
	// Use the raw syscall for this value-only attribute. Converting a numeric
	// HANDLE to a Go pointer is invalid under checkptr (including race builds).
	update := windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")
	if result, _, callErr := update.Call(uintptr(unsafe.Pointer(attrs.List())), 0, windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(t.console), unsafe.Sizeof(t.console), 0, 0); result == 0 {
		return nil, fmt.Errorf("attach ConPTY attribute: %w", callErr)
	}
	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	// NULL standard handles plus STARTF_USESTDHANDLES let ConPTY supply its
	// console handles. Without the flag, Windows can duplicate redirected
	// service stdio into the child even with inheritHandles=false, bypassing
	// the terminal stream (for example when the server logs to a file).
	// https://github.com/microsoft/terminal/discussions/15814
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.StdInput, startup.StdOutput, startup.StdErr = 0, 0, 0
	startup.ProcThreadAttributeList = attrs.List()
	app, err := windows.UTF16PtrFromString(resolved)
	if err != nil {
		return nil, err
	}
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = syscall.EscapeArg(arg)
	}
	line, err := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	if err != nil {
		return nil, err
	}
	dir, err := windows.UTF16PtrFromString(cmd.Dir)
	if err != nil {
		return nil, err
	}
	env, err := windowsEnvironment(cmd.Env)
	if err != nil {
		return nil, err
	}
	var pi windows.ProcessInformation
	err = windows.CreateProcess(app, line, nil, nil, false, windows.CREATE_SUSPENDED|windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT, &env[0], dir, &startup.StartupInfo, &pi)
	runtime.KeepAlive(env)
	if err != nil {
		return nil, fmt.Errorf("start terminal process: %w", err)
	}
	t.process, t.pid = pi.Process, int(pi.ProcessId)
	defer windows.CloseHandle(pi.Thread)
	if err = windows.AssignProcessToJobObject(t.job, pi.Process); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		_, _ = windows.WaitForSingleObject(pi.Process, windows.INFINITE)
		return nil, fmt.Errorf("assign terminal process to job: %w", err)
	}
	cmd.Process, err = os.FindProcess(t.pid)
	if err != nil {
		return nil, err
	}
	windowsTerminals.Store(t.pid, t)
	if _, err = windows.ResumeThread(pi.Thread); err != nil {
		windowsTerminals.Delete(t.pid)
		_ = cmd.Process.Release()
		return nil, err
	}
	return t, nil
}

// CreateProcess resolves the image before applying its working directory,
// unlike exec.Cmd.Start. Resolve explicit relative paths against the session
// directory first, and retain Go's PATHEXT/ErrDot rules for executable lookup.
func windowsExecutable(path, dir string) (string, error) {
	if !filepath.IsAbs(path) {
		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return "", err
			}
		}
		if filepath.VolumeName(path) != "" {
			return "", fmt.Errorf("drive-relative executable path %q is ambiguous; use an absolute path", path)
		}
		if strings.HasPrefix(path, `\`) || strings.HasPrefix(path, "/") {
			path = filepath.VolumeName(dir) + path
		} else {
			path = filepath.Join(dir, path)
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	extensions := os.Getenv("PATHEXT")
	if extensions == "" {
		extensions = ".COM;.EXE;.BAT;.CMD"
	}
	for _, extension := range strings.Split(extensions, ";") {
		if extension != "" && strings.EqualFold(filepath.Ext(absolute), "."+strings.TrimPrefix(extension, ".")) {
			return absolute, nil
		}
	}
	return exec.LookPath(absolute)
}

// newWindowsInputPipe separates the overlapped service writer from the
// synchronous reader required by ConPTY. Neither handle is inheritable.
func newWindowsInputPipe() (windows.Handle, windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\pipe\webmux-%d-%s`, os.Getpid(), randomClientID()))
	if err != nil {
		return 0, 0, err
	}
	writer, err := windows.CreateNamedPipe(name, windows.PIPE_ACCESS_OUTBOUND|windows.FILE_FLAG_OVERLAPPED|windows.FILE_FLAG_FIRST_PIPE_INSTANCE, windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS, 1, 65536, 65536, 0, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("create terminal input: %w", err)
	}
	reader, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		_ = windows.CloseHandle(writer)
		return 0, 0, err
	}
	return writer, reader, nil
}

func windowsEnvironment(entries []string) ([]uint16, error) {
	// Windows environment names are case-insensitive; the last value wins.
	values := make(map[string]string)
	for _, entry := range entries {
		if strings.ContainsRune(entry, 0) {
			return nil, errors.New("environment contains NUL")
		}
		start := 0
		if strings.HasPrefix(entry, "=") {
			start = 1
		} // per-drive current directory
		at := strings.IndexByte(entry[start:], '=')
		if at < 0 {
			continue
		}
		values[strings.ToUpper(entry[:start+at])] = entry
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var block []uint16
	for _, key := range keys {
		value, err := windows.UTF16FromString(values[key])
		if err != nil {
			return nil, err
		}
		block = append(block, value...)
	}
	if len(block) == 0 {
		block = append(block, 0)
	}
	return append(block, 0), nil
}

func (t *windowsTerminal) Read(p []byte) (int, error) { return t.output.Read(p) }
func (t *windowsTerminal) SetWriteDeadline(deadline time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return os.ErrClosed
	}
	t.deadline = deadline
	close(t.changed)
	t.changed = make(chan struct{})
	return nil
}
func (t *windowsTerminal) Write(p []byte) (int, error) {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	t.mu.Lock()
	closed, deadline := t.closed, t.deadline
	t.mu.Unlock()
	if closed {
		return 0, os.ErrClosed
	}
	if !deadline.IsZero() && !time.Now().Before(deadline) {
		return 0, os.ErrDeadlineExceeded
	}
	if len(p) == 0 {
		return 0, nil
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(event)
	ov := windows.Overlapped{HEvent: event}
	var n uint32
	err = windows.WriteFile(t.input, p, &n, &ov)
	if err != windows.ERROR_IO_PENDING {
		return int(n), err
	}
	complete := make(chan struct{})
	go func() { _, _ = windows.WaitForSingleObject(event, windows.INFINITE); close(complete) }()
	var cancelled error
	for {
		t.mu.Lock()
		deadline, changed, closed := t.deadline, t.changed, t.closed
		t.mu.Unlock()
		var timeout <-chan time.Time
		var timer *time.Timer
		if closed {
			cancelled = os.ErrClosed
		} else if !deadline.IsZero() {
			if !time.Now().Before(deadline) {
				cancelled = os.ErrDeadlineExceeded
			} else {
				timer = time.NewTimer(time.Until(deadline))
				timeout = timer.C
			}
		}
		if cancelled != nil {
			_ = windows.CancelIoEx(t.input, &ov)
			<-complete
			break
		}
		select {
		case <-complete:
			if timer != nil {
				timer.Stop()
			}
			err = windows.GetOverlappedResult(t.input, &ov, &n, false)
			runtime.KeepAlive(p)
			return int(n), err
		case <-changed:
		case <-timeout:
		}
		if timer != nil {
			timer.Stop()
		}
	}
	_ = windows.GetOverlappedResult(t.input, &ov, &n, false)
	runtime.KeepAlive(p)
	return int(n), cancelled
}
func (t *windowsTerminal) closeConsole() {
	t.consoleOnce.Do(func() {
		t.consoleMu.Lock()
		console := t.console
		t.console = 0
		t.consoleMu.Unlock()
		// Never hold consoleMu during the synchronous flush: Resize holds the
		// session mutex while consulting consoleMu, and the output reader needs
		// that session mutex to drain the flush.
		if console != 0 {
			windows.ClosePseudoConsole(console)
		}
	})
}
func (t *windowsTerminal) Close() error {
	t.closeOnce.Do(func() {
		if t.job != 0 {
			_ = windows.TerminateJobObject(t.job, 1)
		}
		t.mu.Lock()
		t.closed = true
		close(t.changed)
		t.mu.Unlock()
		t.writeMu.Lock()
		if t.input != 0 {
			_ = windows.CloseHandle(t.input)
		}
		t.writeMu.Unlock()
		// ClosePseudoConsole may synchronously flush output. Keep the reader alive
		// until it finishes rather than holding a session lock across this call.
		t.closeConsole()
		if t.output != nil {
			_ = t.output.Close()
		}
		if t.job != 0 {
			_ = windows.CloseHandle(t.job)
		}
		if t.process != 0 {
			_ = windows.CloseHandle(t.process)
		}
	})
	return nil
}
func resizeTerminal(f terminalIO, cols, rows uint16) error {
	t := f.(*windowsTerminal)
	t.consoleMu.Lock()
	defer t.consoleMu.Unlock()
	if t.console == 0 {
		return os.ErrClosed
	}
	return windows.ResizePseudoConsole(t.console, windows.Coord{X: int16(cols), Y: int16(rows)})
}
func waitTerminal(cmd *exec.Cmd, f terminalIO) int {
	t := f.(*windowsTerminal)
	defer cmd.Process.Release()
	_, err := windows.WaitForSingleObject(t.process, windows.INFINITE)
	var code uint32
	if err != nil || windows.GetExitCodeProcess(t.process, &code) != nil {
		return -1
	}
	return int(code)
}
func hangupTerminal(pid int) { forceSession(pid, "") }
func cleanupProcesses(pid int, _ string) {
	if value, ok := windowsTerminals.LoadAndDelete(pid); ok {
		t := value.(*windowsTerminal)
		_ = windows.TerminateJobObject(t.job, 1)
		t.closeConsole()
	}
}
func forceSession(pid int, _ string) {
	if value, ok := windowsTerminals.Load(pid); ok {
		_ = windows.TerminateJobObject(value.(*windowsTerminal).job, 1)
	}
}
func enableReaper() error            { return nil }
func releaseLeader(_ int)            {}
func reapAdopted(_ processTable)     {}
func startChild(cmd *exec.Cmd) error { return cmd.Start() }

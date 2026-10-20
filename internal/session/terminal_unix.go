//go:build !windows

package session

import (
	"github.com/creack/pty"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// A nonblocking descriptor handed to NewFile participates in Go's poller.
// Duplicating transfers ownership without retaining two File finalizers on one fd.
func pollablePTY(original *os.File) (*os.File, error) {
	syscall.ForkLock.RLock()
	fd, err := syscall.Dup(int(original.Fd()))
	if err == nil {
		syscall.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, err
	}
	if err = syscall.SetNonblock(fd, true); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	name := original.Name()
	if err = original.Close(); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	if err = f.SetReadDeadline(time.Time{}); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func startTerminal(cmd *exec.Cmd, id string) (terminalIO, error) {
	f, err := startPTY(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, err
	}
	pollable, err := pollablePTY(f)
	if err != nil {
		_ = f.Close()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
		releaseLeader(cmd.Process.Pid)
		cleanupProcesses(cmd.Process.Pid, id)
		return nil, err
	}
	return pollable, nil
}
func resizeTerminal(f terminalIO, cols, rows uint16) error {
	return pty.Setsize(f.(*os.File), &pty.Winsize{Cols: cols, Rows: rows})
}
func hangupTerminal(pid int) { _ = syscall.Kill(-pid, syscall.SIGHUP) }
func waitTerminal(cmd *exec.Cmd, _ terminalIO) int {
	if cmd.Wait() == nil {
		return 0
	}
	code := cmd.ProcessState.ExitCode()
	if code < 0 {
		if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			code = 128 + int(status.Signal())
		}
	}
	return code
}

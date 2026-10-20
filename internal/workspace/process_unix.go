//go:build !windows

package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"

	"github.com/rainchan/webmux/internal/session"
)

func runGitCommand(ctx context.Context, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	err := session.RunCommand(cmd)
	// A leader may exit before children close inherited pipes. WaitDelay alone
	// closes the pipes, not the process group.
	if cmd.Process != nil && (errors.Is(err, exec.ErrWaitDelay) || ctx.Err() != nil) {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return err
}

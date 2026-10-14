//go:build !linux && !darwin

package session

import (
	"syscall"
	"time"
)

func enableReaper() error { return nil }
func cleanupProcesses(pid int, _ string) {
	_ = syscall.Kill(-pid, syscall.SIGHUP)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if syscall.Kill(-pid, 0) == syscall.ESRCH {
			return
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			return
		}
	}
}
func forceSession(pid int, _ string) { _ = syscall.Kill(-pid, syscall.SIGKILL) }

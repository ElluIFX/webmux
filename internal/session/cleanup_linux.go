//go:build linux

package session

import (
	"sync"
	"syscall"
)

const prSetChildSubreaper = 36

var reaperOnce sync.Once
var reaperError error

// Own the adopted descendants so killing a shell never leaves zombies to PID 1.
func enableReaper() error {
	reaperOnce.Do(func() {
		_, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prSetChildSubreaper, 1, 0, 0, 0, 0)
		if errno != 0 {
			reaperError = errno
		}
	})
	return reaperError
}

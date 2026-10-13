//go:build linux

package session

import (
	"bytes"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const prSetChildSubreaper = 36

var reaperOnce sync.Once
var reaperError error

func enableReaper() error {
	reaperOnce.Do(func() {
		_, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prSetChildSubreaper, 1, 0, 0, 0, 0)
		if errno != 0 {
			reaperError = errno
		}
	})
	return reaperError
}

// Own the adopted descendants so killing a shell never leaves zombies to PID 1.
func cleanupProcesses(pid int, id string) {
	table := processes()
	targets := make(map[int]uint64)
	marker := []byte("WEBMUX_SESSION_ID=" + id)
	for child, state := range table {
		owned := state.sid == pid
		if !owned && state.state != 'Z' {
			data, err := os.ReadFile("/proc/" + strconv.Itoa(child) + "/environ")
			if err == nil {
				for len(data) > 0 {
					value, rest, _ := bytes.Cut(data, []byte{0})
					if bytes.Equal(value, marker) {
						owned = true
						break
					}
					data = rest
				}
			}
		}
		if owned {
			targets[child] = state.start
		}
	}
	signalTargets(targets, table, syscall.SIGHUP)
	start := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		table = processes()
		// Shell job-control groups share a session ID; include new descendants too.
		for child, state := range table {
			if state.sid == pid {
				targets[child] = state.start
			}
		}
		for changed := true; changed; {
			changed = false
			for child, state := range table {
				if _, known := targets[child]; known {
					continue
				}
				if parentStart, owned := targets[state.ppid]; owned && table[state.ppid].start == parentStart {
					targets[child] = state.start
					changed = true
				}
			}
		}
		for child, born := range targets {
			state, exists := table[child]
			if !exists || state.start != born {
				delete(targets, child)
				continue
			}
			if child != pid && state.state == 'Z' && state.ppid == os.Getpid() {
				process, err := os.FindProcess(child)
				if err == nil {
					_, _ = process.Wait()
				}
				delete(targets, child)
			}
		}
		if len(targets) == 0 {
			return
		}
		if time.Since(start) >= 5*time.Second {
			signalTargets(targets, table, syscall.SIGKILL)
		}
		if time.Since(start) >= 7*time.Second {
			return
		}
		<-ticker.C
	}
}
func signalTargets(targets map[int]uint64, table processTable, signal syscall.Signal) {
	for pid, born := range targets {
		if state, exists := table[pid]; exists && state.start == born && state.state != 'Z' {
			_ = syscall.Kill(pid, signal)
		}
	}
}
func forceSession(pid int, id string) {
	table := processes()
	targets := make(map[int]uint64)
	for child, state := range table {
		if state.sid == pid {
			targets[child] = state.start
		}
	}
	signalTargets(targets, table, syscall.SIGKILL)
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

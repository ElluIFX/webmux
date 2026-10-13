package session

import "syscall"

func killGroup(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) }

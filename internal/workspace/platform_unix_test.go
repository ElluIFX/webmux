//go:build !windows

package workspace

import (
	"syscall"
	"testing"
)

func requireUnixFixture(t *testing.T)       { t.Helper() }
func testRepositoryName() string            { return "repo with space\nand newline" }
func testPrintCommand(text string) []string { return []string{"/bin/sh", "-c", "printf " + text} }
func testProcessGone(pid int) error {
	err := syscall.Kill(pid, 0)
	if err == syscall.ESRCH {
		return nil
	}
	if err == nil {
		return syscall.EEXIST
	}
	return err
}

func testFilename(name string) string { return name }

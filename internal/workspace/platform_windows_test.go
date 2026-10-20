package workspace

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireUnixFixture(t *testing.T) {
	t.Helper()
	t.Skip("POSIX shell/job-control fixture; Windows lifecycle coverage is in windows_test.go")
}
func testRepositoryName() string { return "repo with space and Unicode 目录" }
func testPrintCommand(text string) []string {
	return []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"), "/d", "/c", "echo " + text}
}
func testProcessGone(pid int) error {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err == windows.ERROR_INVALID_PARAMETER {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	result, err := windows.WaitForSingleObject(h, 0)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("process %d is still running", pid)
	}
	return nil
}

func testFilename(name string) string { return strings.ReplaceAll(name, "\n", " ") }

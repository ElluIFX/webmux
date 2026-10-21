package codeview

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if name == "git" && os.Getenv("WEBMUX_CODEVIEW_GIT_HELPER") == "1" {
		os.Exit(runGitTestHelper())
	}
	os.Exit(m.Run())
}

func runGitTestHelper() int {
	if log := os.Getenv("WEBMUX_CODEVIEW_GIT_LOG"); log != "" {
		f, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_, err = f.WriteString("call\n")
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return 1
		}
	}
	if records := os.Getenv("WEBMUX_CODEVIEW_GIT_RECORDS"); records != "" && slices.Contains(os.Args[1:], "ls-files") {
		f, err := os.Open(records)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer f.Close()
		if _, err := io.Copy(os.Stdout, f); err != nil {
			return 1
		}
		return 0
	}
	cmd := exec.Command(os.Getenv("WEBMUX_CODEVIEW_REAL_GIT"), os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// Re-execute the native test binary as Git on every platform, preserving argv
// and binary output without depending on a shell or command-file quoting.
func installGitTestHelper(t *testing.T, log, records string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	bin := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target, err := os.OpenFile(filepath.Join(bin, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(target, source)
	closeErr := target.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("copy native Git test helper: %v, %v", err, closeErr)
	}
	t.Setenv("WEBMUX_CODEVIEW_GIT_HELPER", "1")
	t.Setenv("WEBMUX_CODEVIEW_REAL_GIT", realGit)
	t.Setenv("WEBMUX_CODEVIEW_GIT_LOG", log)
	t.Setenv("WEBMUX_CODEVIEW_GIT_RECORDS", records)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

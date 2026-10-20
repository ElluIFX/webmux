//go:build windows

package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWindowsRegistryLockAndReplacement(t *testing.T) {
	isolateGit(t)
	root := initRepo(t)
	state := t.TempDir()
	s, manager := testService(t, state, root)
	r := registerTest(t, s, root)
	other := New(state, manager, root)
	if other.Capability().RegistryAvailable {
		t.Fatal("second writer acquired registry lock")
	}
	other.Close()
	// Replacing an existing file must work repeatedly, not only on first save.
	if err := s.Unregister(r.ID); err != nil {
		t.Fatal(err)
	}
	r = registerTest(t, s, root)
	s.Close()
	reopened := New(state, manager, root)
	defer reopened.Close()
	list, err := reopened.List(context.Background())
	if err != nil || len(list) != 1 || list[0].ID != r.ID {
		t.Fatalf("persisted registry: %+v %v", list, err)
	}
	duplicate, created, err := reopened.Register(context.Background(), strings.ToUpper(root))
	if err != nil || created || duplicate.ID != r.ID {
		t.Fatalf("case alias duplicated registry: %+v %v %v", duplicate, created, err)
	}
}

func TestWindowsWorktreeCreateReadAndRemove(t *testing.T) {
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	tree := linkedTest(t, s, r, "windows-native")
	data, err := ReadGit(context.Background(), tree.Path, 1024, "show", "HEAD:tracked")
	if err != nil || string(data) != "original\n" {
		t.Fatalf("read Git: %q %v", data, err)
	}
	putFile(t, filepath.Join(tree.Path, "tracked"), "dirty\n")
	preview := previewTest(t, s, r, tree)
	if err := s.RemoveWorktree(context.Background(), r.ID, tree.ID, RemoveRequest{ConfirmationID: preview.ConfirmationID, Mode: "safe"}); Status(err) != 409 {
		t.Fatalf("safe deletion discarded dirty data: %v", err)
	}
	assertExists(t, tree.Path)
	preview = previewTest(t, s, r, tree)
	if err := s.RemoveWorktree(context.Background(), r.ID, tree.ID, forceRequest(preview)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tree.Path); !os.IsNotExist(err) {
		t.Fatalf("worktree survived: %v", err)
	}
	gitTest(t, root, "show-ref", "--verify", "refs/heads/windows-native")
}

func TestWindowsGitHelper(t *testing.T) {
	mode := os.Getenv("WEBMUX_WORKSPACE_HELPER")
	if mode == "" {
		return
	}
	if mode == "child" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	if mode == "overflow" {
		block := strings.Repeat("x", 4096)
		for {
			fmt.Print(block)
		}
	}
	child := exec.Command(os.Args[0], "-test.run=^TestWindowsGitHelper$")
	child.Env = append(os.Environ(), "WEBMUX_WORKSPACE_HELPER=child")
	// Deliberately inherit pipes and outlive the leader.
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("WEBMUX_WORKSPACE_PID_FILE"), []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
		os.Exit(3)
	}
	if mode == "leader-exits" {
		os.Exit(0)
	}
	time.Sleep(time.Minute)
	os.Exit(0)
}

func TestWindowsGitCancellationKillsDescendants(t *testing.T) {
	for _, mode := range []string{"leader-waits", "leader-exits"} {
		t.Run(mode, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsGitHelper$")
			cmd.Env = append(os.Environ(), "WEBMUX_WORKSPACE_HELPER="+mode, "WEBMUX_WORKSPACE_PID_FILE="+pidFile)
			cmd.WaitDelay = 100 * time.Millisecond
			cmd.Stdout, cmd.Stderr = &boundedOutput{limit: 1024, cancel: cancel}, &boundedOutput{limit: 1024, cancel: cancel}
			done := make(chan error, 1)
			go func() { done <- runGitCommand(ctx, cmd) }()
			deadline := time.Now().Add(10 * time.Second)
			var pid int
			for time.Now().Before(deadline) {
				if data, err := os.ReadFile(pidFile); err == nil {
					pid, _ = strconv.Atoi(string(data))
					if pid != 0 {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid == 0 {
				cancel()
				t.Fatal("helper did not create descendant")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("Git cancellation blocked")
			}
			for time.Now().Before(deadline) {
				if testProcessGone(pid) == nil {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("Git descendant survived job cleanup")
		})
	}
}

func TestWindowsGitOutputLimitCancelsProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsGitHelper$")
	cmd.Env = append(os.Environ(), "WEBMUX_WORKSPACE_HELPER=overflow")
	out := &boundedOutput{limit: 8192, cancel: cancel}
	cmd.Stdout = out
	cmd.WaitDelay = time.Second
	start := time.Now()
	if err := runGitCommand(ctx, cmd); err == nil || !out.exceeded || out.Len() > out.limit {
		t.Fatalf("output bound failed: err=%v exceeded=%v bytes=%d", err, out.exceeded, out.Len())
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("output limit failed to terminate process promptly")
	}
}

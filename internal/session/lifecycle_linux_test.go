//go:build linux

package session

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestAdoptedZombieReapedWhileLeaderLives(t *testing.T) {
	m := NewManager(Options{DefaultCwd: t.TempDir(), Presets: []Preset{{Name: "daemon", Cmd: []string{"/bin/bash", "-c", `setsid /bin/bash -c 'sleep 0.05 & printf "DAEMON_CHILD=%s\n" "$!"'; read -r stop; exit 23`}}}})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	s, err := m.Create("", "daemon", nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`DAEMON_CHILD=(\d+)`)
	var match []string
	until := time.Now().Add(time.Second)
	for len(match) == 0 {
		match = pattern.FindStringSubmatch(string(s.Buffer()))
		if time.Now().After(until) {
			t.Fatal("daemon child was not announced")
		}
		time.Sleep(10 * time.Millisecond)
	}
	child, _ := strconv.Atoi(match[1])
	defer func() { var status syscall.WaitStatus; _, _ = syscall.Wait4(child, &status, syscall.WNOHANG, nil) }()
	until = time.Now().Add(3 * time.Second)
	for {
		_, err = os.Stat("/proc/" + strconv.Itoa(child))
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(until) {
			t.Fatal("adopted new-session zombie remains while manager is alive")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if s.Info().Status == StatusExited {
		t.Fatal("reaper consumed the live session leader")
	}
	if err = s.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("leader wait did not finish")
	}
	if info := s.Info(); info.ExitCode == nil || *info.ExitCode != 23 {
		t.Fatalf("leader wait result stolen: %+v", info)
	}
}

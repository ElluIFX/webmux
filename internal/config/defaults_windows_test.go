package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWindowsDefaultsAndShellArguments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", filepath.Join(home, "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "Local"))
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("PATH", home)
	t.Setenv("ProgramW6432", home)
	t.Setenv("ProgramFiles", home)
	t.Setenv("ProgramFiles(x86)", home)
	if got := DefaultPath(); got != filepath.Join(home, "Roaming", "webmux", "config.toml") {
		t.Fatal(got)
	}
	if got := DefaultStateDir(); got != filepath.Join(home, "Local", "webmux") {
		t.Fatal(got)
	}
	for _, row := range []struct {
		shell string
		args  []string
	}{
		{"powershell.exe", []string{"-NoLogo", "-NoExit"}},
		{"POWERSHELL.EXE", []string{"-NoLogo", "-NoExit"}},
		{"CMD.EXE", []string{"/D"}},
		{`C:\Program Files\PowerShell\7\pwsh.exe`, []string{"-NoLogo", "-NoExit"}},
		{"cmd.exe", []string{"/D"}},
		{"bash.exe", []string{"-l", "-i"}},
	} {
		c, err := Load(filepath.Join(home, "absent"), false)
		if err != nil {
			t.Fatal(err)
		}
		c.Shell = row.shell
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c.Presets[0].Cmd, append([]string{row.shell}, row.args...)) {
			t.Fatal(c.Presets[0].Cmd)
		}
	}
	c, _ := Load(filepath.Join(home, "absent"), false)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(c.Shell) != "powershell.exe" {
		t.Fatal(c.Shell)
	}
}

func TestWindowsPowerShellDiscovery(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	writeShell := func(base, version string) string {
		t.Helper()
		path := filepath.Join(base, "PowerShell", version, "pwsh.exe")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	preview := writeShell(root, "10-preview")
	if got := installedPowerShell([]string{"", root}); got != preview {
		t.Fatalf("preview fallback: %q", got)
	}
	writeShell(root, "6")
	stable := writeShell(root, "7")
	writeShell(other, "7")
	if got := installedPowerShell([]string{root, other}); got != stable {
		t.Fatalf("stable native installation: %q", got)
	}
	writeShell(root, "7.9")
	latest := writeShell(root, "7.10")
	writeShell(root, "invalid")
	if got := installedPowerShell([]string{root}); got != latest {
		t.Fatalf("numeric version ordering: %q", got)
	}
	t.Setenv("PATH", filepath.Dir(stable))
	t.Setenv("ProgramFiles", root)
	if got := defaultShell(); got != stable {
		t.Fatalf("PATH preference: %q", got)
	}
}

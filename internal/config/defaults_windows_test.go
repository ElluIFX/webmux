package config

import (
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

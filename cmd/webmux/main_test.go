package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestConfigurationPrecedence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell environment precedence; Windows defaults covered by config tests")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("SHELL", "/bin/sh")
	file := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(file, []byte("listen = '127.0.0.1:9000'\ntoken = 'file-token'\nshell = '/bin/bash'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name                 string
		args                 []string
		listen, token, shell string
	}{
		{"defaults", nil, "127.0.0.1:7681", "", "/bin/sh"},
		{"file", []string{"--config", file}, "127.0.0.1:9000", "file-token", "/bin/bash"},
		{"flags", []string{"--config", file, "--listen", "127.0.0.1:9001", "--token", "flag-token"}, "127.0.0.1:9001", "flag-token", "/bin/bash"},
		{"explicit empty token", []string{"--config", file, "--token="}, "127.0.0.1:9000", "", "/bin/bash"},
	} {
		t.Run(row.name, func(t *testing.T) {
			c, err := loadConfig(row.args)
			if err != nil {
				t.Fatal(err)
			}
			if c.Listen != row.listen || c.Token != row.token || c.Shell != row.shell {
				t.Fatalf("configuration: listen=%q token=%q shell=%q", c.Listen, c.Token, c.Shell)
			}
			if c.DefaultCwd != dir || !reflect.DeepEqual(c.Presets[0].Cmd, []string{row.shell, "-l", "-i"}) {
				t.Fatalf("unresolved defaults: cwd=%q shell argv=%q", c.DefaultCwd, c.Presets[0].Cmd)
			}
		})
	}
}

func TestExplicitConfigurationAndOverridesAreValidated(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadConfig([]string{"--config", filepath.Join(dir, "missing.toml")}); err == nil {
		t.Fatal("explicit missing config was ignored")
	}
	file := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(file, []byte("listen = 'invalid'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig([]string{"--config", file}); err == nil {
		t.Fatal("invalid file listen accepted")
	}
	if _, err := loadConfig([]string{"--config", file, "--listen", "127.0.0.1:9000"}); err != nil {
		t.Fatalf("file validated before flag override: %v", err)
	}
	if _, err := loadConfig([]string{"--config", file, "--listen="}); err == nil {
		t.Fatal("explicit empty listen accepted")
	}
}

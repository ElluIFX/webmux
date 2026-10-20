//go:build !windows

package config

import (
	"os"
	"path/filepath"
)

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "webmux", "config.toml")
}
func defaultStateDir() string {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "webmux")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "webmux")
}
func defaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash"
}
func shellCommand(shell string) []string { return []string{shell, "-l", "-i"} }

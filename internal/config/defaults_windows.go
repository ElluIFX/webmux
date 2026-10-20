package config

import (
	"os"
	"path/filepath"
	"strings"
)

func defaultConfigPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base, _ = os.UserHomeDir()
	}
	return filepath.Join(base, "webmux", "config.toml")
}
func defaultStateDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "webmux")
}
func defaultShell() string {
	// Windows PowerShell ships with every supported Windows release. Do not
	// inherit SHELL from Git Bash: it commonly names a Unix-only /bin/bash.
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}
func shellCommand(shell string) []string {
	switch strings.TrimSuffix(strings.ToLower(filepath.Base(shell)), ".exe") {
	case "powershell", "pwsh":
		return []string{shell, "-NoLogo", "-NoExit"}
	case "cmd":
		return []string{shell, "/D"}
	case "bash", "sh", "zsh", "fish":
		return []string{shell, "-l", "-i"}
	default:
		return []string{shell}
	}
}

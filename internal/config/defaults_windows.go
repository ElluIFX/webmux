package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
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
	// PATH also covers portable installations and Microsoft Store aliases.
	// Ignore ErrDot: a shell in the server's current directory is not a default.
	if path, err := exec.LookPath("pwsh.exe"); err == nil {
		return path
	}
	if path := installedPowerShell([]string{os.Getenv("ProgramW6432"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")}); path != "" {
		return path
	}
	// Do not inherit SHELL from Git Bash, which commonly names /bin/bash.
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}

func installedPowerShell(roots []string) string {
	type candidate struct {
		path    string
		version []int
		preview bool
	}
	var candidates []candidate
	for _, root := range roots {
		if root == "" || !filepath.IsAbs(root) {
			continue
		}
		base := filepath.Join(root, "PowerShell")
		entries, _ := os.ReadDir(base)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			number, _, preview := strings.Cut(name, "-")
			var version []int
			for _, part := range strings.Split(number, ".") {
				n, err := strconv.Atoi(part)
				if err != nil || n < 0 {
					version = nil
					break
				}
				version = append(version, n)
			}
			if len(version) == 0 || version[0] < 6 {
				continue
			}
			path := filepath.Join(base, entry.Name(), "pwsh.exe")
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				candidates = append(candidates, candidate{path, version, preview})
			}
		}
	}
	// Stable versions precede previews, then the highest numeric version wins.
	// Stable sorting retains the native Program Files directory on ties.
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		if a.preview != b.preview {
			if a.preview {
				return 1
			}
			return -1
		}
		return slices.Compare(b.version, a.version)
	})
	if len(candidates) > 0 {
		return candidates[0].path
	}
	return ""
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

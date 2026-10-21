package session

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Recognize the complete Node shim emitted by npm's cmd-shim, including the
// two placements of its PATHEXT adjustment. Custom batch logic must not be
// silently discarded. Execute the interpreter directly to preserve literal argv.
var npmNodeShim = regexp.MustCompile(`(?i)\A@ECHO off\nGOTO start\n:find_dp0\nSET dp0=%~dp0\nEXIT /b\n:start\nSETLOCAL\nCALL :find_dp0\n\nIF EXIST "%dp0%\\node.exe" \(\n  SET "_prog=%dp0%\\node.exe"\n\) ELSE \(\n  SET "_prog=node"\n(  SET PATHEXT=%PATHEXT:;.JS;=;%\n)?\)\n\nendLocal & goto #_undefined_# 2>NUL \|\| title %COMSPEC% & (set PATHEXT=%PATHEXT:;.JS;=;% & )?"%_prog%"  "%dp0%\\([^"%\r\n]+)" %\*\n?\z`)

func windowsLauncher(path string, args []string) (string, []string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".cmd" && ext != ".bat" {
		return path, args, nil
	}
	unsupported := fmt.Errorf("Windows batch launcher %q requires an explicit interpreter; use a standard npm Node launcher, or configure node.exe with the CLI entrypoint", path)
	f, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("read launcher %q: %w", path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16*1024+1))
	if err != nil {
		return "", nil, err
	}
	if len(data) > 16*1024 {
		return "", nil, unsupported
	}
	match := npmNodeShim.FindStringSubmatch(strings.ReplaceAll(string(data), "\r\n", "\n"))
	if match == nil || (match[1] == "") == (match[2] == "") || strings.ContainsAny(match[3], ":") || strings.HasPrefix(match[3], `\`) {
		return "", nil, unsupported
	}
	entry := filepath.Join(filepath.Dir(path), match[3])
	if info, err := os.Stat(entry); err != nil || info.IsDir() {
		return "", nil, fmt.Errorf("npm launcher %q has a missing entrypoint %q; reinstall the CLI package", path, entry)
	}
	node := filepath.Join(filepath.Dir(path), "node.exe")
	if info, err := os.Stat(node); err != nil || info.IsDir() {
		node, err = exec.LookPath("node.exe")
		if err != nil {
			return "", nil, fmt.Errorf("npm launcher %q requires Node.js on PATH: %w", path, err)
		}
	}
	argv := append([]string{node, entry}, args[1:]...)
	return node, argv, nil
}

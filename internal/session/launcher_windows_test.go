package session

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const npmShimFixture = `@ECHO off
GOTO start
:find_dp0
SET dp0=%~dp0
EXIT /b
:start
SETLOCAL
CALL :find_dp0

IF EXIST "%dp0%\node.exe" (
  SET "_prog=%dp0%\node.exe"
) ELSE (
  SET "_prog=node"
  SET PATHEXT=%PATHEXT:;.JS;=;%
)

endLocal & goto #_undefined_# 2>NUL || title %COMSPEC% & "%_prog%"  "%dp0%\node_modules\fixture\cli.js" %*
`

func writeNpmFixture(t *testing.T, shim string) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "fixture.cmd")
	entry := filepath.Join(root, "node_modules", "fixture", "cli.js")
	if err := os.MkdirAll(filepath.Dir(entry), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(shim, "\n", "\r\n")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`require('node:fs').writeFileSync(process.argv[2], JSON.stringify(process.argv.slice(3)));`), 0600); err != nil {
		t.Fatal(err)
	}
	return path, entry
}

func TestWindowsNpmLaunchPreservesLiteralArguments(t *testing.T) {
	if _, err := exec.LookPath("node.exe"); err != nil {
		t.Skip("Node.js is required for the npm launcher integration test")
	}
	current := strings.Replace(npmShimFixture, "  SET PATHEXT=%PATHEXT:;.JS;=;%\n", "", 1)
	current = strings.Replace(current, `& "%_prog%"`, `& set PATHEXT=%PATHEXT:;.JS;=;% & "%_prog%"`, 1)
	for name, shim := range map[string]string{"legacy": npmShimFixture, "current": current} {
		t.Run(name, func(t *testing.T) {
			path, _ := writeNpmFixture(t, shim)
			output := filepath.Join(t.TempDir(), "argv.json")
			want := []string{"", `C:\my work\`, `a"b`, "%PATH%", "!value!", "a & b | c > d", "中文", "line\nbreak"}
			s := newIdleSession(t, append([]string{path, output}, want...)...)
			waitWindowsExit(t, s, 0)
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			if err := json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("argv = %q, error = %v; want %q", got, err, want)
			}
		})
	}
}

func TestWindowsNpmLauncherDependenciesAndCustomLogic(t *testing.T) {
	path, entry := writeNpmFixture(t, npmShimFixture)
	t.Setenv("PATH", t.TempDir())
	if _, _, err := windowsLauncher(path, []string{path}); err == nil || !strings.Contains(err.Error(), "requires Node.js") {
		t.Fatalf("missing Node.js: %v", err)
	}
	node := filepath.Join(filepath.Dir(path), "node.exe")
	if err := os.WriteFile(node, nil, 0600); err != nil {
		t.Fatal(err)
	}
	resolved, argv, err := windowsLauncher(path, []string{path, "value"})
	if err != nil || resolved != node || !reflect.DeepEqual(argv, []string{node, entry, "value"}) {
		t.Fatalf("adjacent Node.js: %q, %q, %v", resolved, argv, err)
	}
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	if _, _, err := windowsLauncher(path, []string{path}); err == nil || !strings.Contains(err.Error(), "missing entrypoint") {
		t.Fatalf("missing package: %v", err)
	}
	for _, script := range []string{
		"@echo custom setup\n" + npmShimFixture,
		npmShimFixture + "echo custom cleanup\n",
		strings.Replace(npmShimFixture, `"%_prog%"  `, `"%_prog%" --experimental-strip-types `, 1),
		strings.Replace(npmShimFixture, `SET "_prog=node"`, `SET "_prog=python"`, 1),
	} {
		path, _ := writeNpmFixture(t, script)
		if _, _, err := windowsLauncher(path, []string{path}); err == nil || !strings.Contains(err.Error(), "explicit interpreter") {
			t.Fatalf("custom script accepted: %v", err)
		}
	}
}

func TestWindowsAbsoluteDirectoryPaths(t *testing.T) {
	for input, want := range map[string]string{
		`C:`: `C:\`, `C:/work/../projects`: `C:\projects`, `\\server\share\folder`: `\\server\share\folder`,
	} {
		if got, err := ExpandPath(input); err != nil || got != want {
			t.Fatalf("ExpandPath(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := ExpandPath(`C:relative`); err == nil {
		t.Fatal("ambiguous drive-relative directory accepted")
	}
}

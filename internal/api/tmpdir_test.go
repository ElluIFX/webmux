package api

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// macOS symlinks and Windows 8.3 TEMP aliases differ from the canonical
// paths reported by sessions and worktrees. Create fixtures in that same form.
func TestMain(m *testing.M) {
	if dir, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		_ = os.Setenv("TMPDIR", dir)
		if runtime.GOOS == "windows" {
			_ = os.Setenv("TMP", dir)
			_ = os.Setenv("TEMP", dir)
		}
	}
	os.Exit(m.Run())
}

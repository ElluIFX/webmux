package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// macOS places TMPDIR behind the /var -> /private/var symlink. Windows
// runners may use an 8.3 alias such as RUNNER~1 in TEMP/TMP.
// Sessions and worktrees report canonical paths. Create fixtures under that
// same canonical root so path and stable-ID assertions compare like for like.
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

package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// macOS places TMPDIR behind the /var -> /private/var symlink, while sessions
// and worktrees report canonical paths. Compare against the canonical form.
func TestMain(m *testing.M) {
	if dir, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		_ = os.Setenv("TMPDIR", dir)
	}
	os.Exit(m.Run())
}

package workspace

import (
	"path/filepath"
	"strings"
)

// Windows paths are case insensitive by default, including Git's path output.
// Treat differently cased spellings conservatively as the same workspace for
// overlap checks, registry deduplication, and stable identifiers.
func pathKey(path string) string { return strings.ToLower(filepath.Clean(path)) }

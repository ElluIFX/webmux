package session

import (
	"fmt"
	"path/filepath"
	"strings"
)

func absolutePath(path string) (string, error) {
	volume := filepath.VolumeName(path)
	if len(volume) == 2 && volume[1] == ':' {
		if path == volume {
			path += `\`
		} else if !strings.ContainsAny(path[2:3], `\/`) {
			return "", fmt.Errorf("drive-relative path %q is ambiguous; use an absolute path such as %s\\", path, volume)
		}
	}
	return filepath.Abs(path)
}

//go:build !windows

package api

import "os"

func privateUploadDir() (string, error) { return os.MkdirTemp("", "webmux-upload-") }

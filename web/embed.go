package web

import "embed"

// Assets contains production resources built by pnpm before go build.
//
//go:embed all:dist
var Assets embed.FS

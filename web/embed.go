//go:build embed

// Package web provides the built UI (web/dist). With -tags embed it is included in the executable:
// `npm --prefix web run build && go build -tags embed ./cmd/observatoire`.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Interface returns the UI files.
func Interface() (fs.FS, error) { return fs.Sub(dist, "dist") }

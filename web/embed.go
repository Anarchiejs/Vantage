//go:build embed

// Package web fournit l'interface compilée (web/dist). Avec -tags embed elle est intégrée à l'exécutable :
// `npm --prefix web run build && go build -tags embed ./cmd/observatoire`.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Interface renvoie les fichiers de l'interface.
func Interface() (fs.FS, error) { return fs.Sub(dist, "dist") }

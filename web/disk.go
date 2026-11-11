//go:build !embed

package web

import (
	"io/fs"
	"os"
)

// Interface reads web/dist from disk (development, tests); nil if the UI has not been built.
func Interface() (fs.FS, error) {
	if _, err := os.Stat("web/dist/index.html"); err != nil {
		return nil, nil
	}
	return os.DirFS("web/dist"), nil
}

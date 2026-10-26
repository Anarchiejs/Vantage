//go:build !embed

package web

import (
	"io/fs"
	"os"
)

// Interface lit web/dist sur le disque (développement, tests) ; nil si l'interface n'a pas été construite.
func Interface() (fs.FS, error) {
	if _, err := os.Stat("web/dist/index.html"); err != nil {
		return nil, nil
	}
	return os.DirFS("web/dist"), nil
}

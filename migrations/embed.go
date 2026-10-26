// Package migrations embarque les fichiers SQL de goose dans l'exécutable.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

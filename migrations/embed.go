// Package migrations embeds the goose SQL files in the executable.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

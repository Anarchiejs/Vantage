// Command make-repos rebuilds the two deterministic Git repositories used by tests.
// go test ./... creates them when they are missing; run this command to rebuild them.
package main

import (
	"fmt"
	"os"

	"github.com/Anarchiejs/Vantage/internal/fixtures"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	root, err := fixtures.ModuleRoot(wd)
	if err != nil {
		fatal(err)
	}
	if err := fixtures.Build(root); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

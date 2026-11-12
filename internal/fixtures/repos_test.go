package fixtures

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureIsIdempotent(t *testing.T) {
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		git := filepath.Join(root, "testdata", "repos", name, ".git")
		if _, err := os.Stat(git); err != nil {
			t.Fatal(err)
		}
	}
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
}

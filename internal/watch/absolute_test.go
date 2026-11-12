package watch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAbsoluteSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	got := absolute(link)
	absLink, err := filepath.Abs(link)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		resolved, err := filepath.EvalSymlinks(absLink)
		if err != nil {
			t.Fatal(err)
		}
		if got != normalize(resolved) {
			t.Fatalf("got %s, want %s", got, normalize(resolved))
		}
		return
	}
	if got != absLink {
		t.Fatalf("got %s, want %s", got, absLink)
	}
}

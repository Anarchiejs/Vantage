package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseRepos(t *testing.T) {
	base := t.TempDir()
	got, err := parseRepos("# comment\n\nAtlas=../repos/alpha, Compass=../repos/beta\n", base)
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := filepath.Abs(filepath.Join(base, "..", "repos", "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	beta, err := filepath.Abs(filepath.Join(base, "..", "repos", "beta"))
	if err != nil {
		t.Fatal(err)
	}
	if got["Atlas"] != alpha || got["Compass"] != beta || len(got) != 2 {
		t.Fatalf("%v", got)
	}
	if _, err := parseRepos("no-equals", ""); err == nil {
		t.Fatal("expected an error")
	}
	empty, err := parseRepos("", "")
	if err != nil || len(empty) != 0 {
		t.Fatalf("%v %v", empty, err)
	}
}

func TestSampleVaultRemap(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	vault := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "vault")
	got, err := reposForVault(vault, nil)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	alpha, err := filepath.Abs(filepath.Join(root, "testdata", "repos", "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	beta, err := filepath.Abs(filepath.Join(root, "testdata", "repos", "beta"))
	if err != nil {
		t.Fatal(err)
	}
	if got["Atlas"] != alpha || got["Compass"] != beta {
		t.Fatalf("Atlas=%s Compass=%s", got["Atlas"], got["Compass"])
	}
	override := filepath.Join(t.TempDir(), "compass")
	got, err = reposForVault(vault, repoFlags{"Compass=" + override})
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(override)
	if err != nil {
		t.Fatal(err)
	}
	if got["Compass"] != want || got["Atlas"] != alpha {
		t.Fatalf("override: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(vault, ".vantage", "repos")); err != nil {
		t.Fatal(err)
	}
}

func TestRepoFileKeepsCommas(t *testing.T) {
	base := t.TempDir()
	got, err := parseRepoFile("Atlas=../repos/alpha, the copy\n", base)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(filepath.Join(base, "..", "repos", "alpha, the copy"))
	if err != nil {
		t.Fatal(err)
	}
	if got["Atlas"] != want || len(got) != 1 {
		t.Fatalf("%v", got)
	}
	flagged, err := parseRepos(`Atlas=C:\a\,b, Compass=C:\c`, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(flagged) != 2 || !strings.HasSuffix(flagged["Atlas"], "a,b") {
		t.Fatalf("%v", flagged)
	}
}

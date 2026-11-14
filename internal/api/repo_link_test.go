package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetRepoLine(t *testing.T) {
	vault := t.TempDir()
	file := filepath.Join(vault, ".vantage", "repos")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(file, []byte("# comment\r\nAtlas=../old\nCompass=../beta\nAtlas=../older\n"), 0o644)
	if err := setRepoLine(vault, "Atlas", "/code/atlas"); err != nil {
		t.Fatal(err)
	}
	if err := setRepoLine(vault, "Draft", "/code/draft"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(file)
	want := "# comment\nAtlas=/code/atlas\nCompass=../beta\n# Atlas=../older\nDraft=/code/draft\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSetRepoLineCarriesLegacyFile(t *testing.T) {
	vault := t.TempDir()
	legacy := filepath.Join(vault, ".observatoire", "repos")
	os.MkdirAll(filepath.Dir(legacy), 0o755)
	os.WriteFile(legacy, []byte("Compass=../beta\n"), 0o644)
	if err := setRepoLine(vault, "Atlas", "/code/atlas"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(vault, ".vantage", "repos"))
	if string(got) != "Compass=../beta\nAtlas=/code/atlas\n" {
		t.Fatalf("got %q", got)
	}
}

func TestCheckRepoFolder(t *testing.T) {
	plain := t.TempDir()
	repo := t.TempDir()
	os.Mkdir(filepath.Join(repo, ".git"), 0o755)
	for _, c := range []struct {
		path, err string
	}{
		{"", "give the folder path"},
		{"relative/dir", "absolute path"},
		{filepath.Join(plain, "missing"), "folder not found"},
		{plain, "not a Git repository"},
	} {
		if _, err := checkRepoFolder(c.path); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%q: got %v, want %q", c.path, err, c.err)
		}
	}
	if got, err := checkRepoFolder(` "` + repo + `" `); err != nil || got != filepath.Clean(repo) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestStatusAndLinkingDisabled(t *testing.T) {
	h := serveurFixtures(t)
	status := getJSON[Status](t, h, "/api/status", http.StatusOK)
	if status.Repositories != 2 || status.Notes == 0 || status.CanLinkRepos {
		t.Fatalf("status %+v", status)
	}
	rec := callHandler(h, "POST", "/api/projects/1/repo", "127.0.0.1", `{"path":"/tmp"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestNoteProject(t *testing.T) {
	h := serveurFixtures(t)
	graph := getJSON[Graph](t, h, "/api/graph", http.StatusOK)
	ids := map[string]int64{}
	for _, node := range graph.Nodes {
		ids[node.Title] = node.ID
	}
	for title, want := range map[string]string{"Atlas - Detail": "Atlas", "Compass": "Compass", "Mapping": ""} {
		note := getJSON[NoteDetail](t, h, "/api/notes/"+itoa(ids[title]), http.StatusOK)
		got := ""
		if note.Project != nil {
			got = note.Project.Name
		}
		if got != want {
			t.Errorf("%s: project %q, want %q", title, got, want)
		}
	}
}

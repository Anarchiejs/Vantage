package vault

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Anarchiejs/Vantage/internal/model"
)

func TestReadFixture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "vault")
	r, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Notes) != 14 || len(r.Cards) != 2 || len(r.Sessions) != 2 || len(r.Journal) != 2 {
		t.Fatalf("counts notes/cards/sessions/journal = %d/%d/%d/%d", len(r.Notes), len(r.Cards), len(r.Sessions), len(r.Journal))
	}
	want := []model.ProjectCard{
		{Name: "Atlas", Status: "active", Updated: "2026-10-26", VerifiedOn: "2026-10-26", VerifiedCommit: "a1b2c3d", Technologies: []string{"Go", "TypeScript"}},
		{Name: "Compass", Status: "archived", SourcePath: "C:/Fixtures/Compass", Updated: "2026-07-01", VerifiedOn: "2026-07-01", VerifiedCommit: "deadbeef", Technologies: []string{"Markdown"}},
	}
	if !reflect.DeepEqual(r.Cards, want) {
		t.Fatalf("cards: %#v", r.Cards)
	}
	for _, s := range r.Sessions {
		if s.AI != "Claude Code" || (s.Project != "Atlas" && s.Project != "Compass") || !strings.HasPrefix(s.NotePath, "Sessions/") || s.Date == "" {
			t.Fatalf("session: %#v", s)
		}
	}
	if r.Journal[0] != (model.JournalEntry{Date: "2026-10-26", AI: "Claude Code", Text: "preparing the fictional fixtures with [[Atlas]]."}) {
		t.Fatalf("journal: %#v", r.Journal)
	}
	foundOrphan, foundInvalid := false, false
	for _, n := range r.Notes {
		if strings.Contains(n.Path, "\\") {
			t.Fatalf("path: %s", n.Path)
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if n.Hash != fmt.Sprintf("%x", sha256.Sum256(raw)) {
			t.Fatal("incorrect raw hash")
		}
		single, err := ReadNote(root, n.Path)
		if err != nil || !reflect.DeepEqual(n, single) {
			t.Fatalf("ReadNote mismatch: %v", err)
		}
		switch n.Title {
		case "Naïve":
			foundOrphan = true
		case "Draft":
			foundInvalid = true
			if n.Type != "" || n.Updated != "" || !strings.HasPrefix(n.Content, "# Draft") {
				t.Fatalf("invalid YAML: %#v", n)
			}
		case "Atlas":
			links := []string{"Atlas - Summary", "Atlas - Detail", "Sessions/2026-10-20 - Atlas - initial import", "Mapping", "Missing note", "Note"}
			if !reflect.DeepEqual(n.Links, links) || n.Folder != "Projects" {
				t.Fatalf("Atlas: %#v", n)
			}
		}
	}
	if !foundOrphan || !foundInvalid {
		t.Fatal("orphan or invalid note lost")
	}
}

func TestLinks(t *testing.T) {
	body := "[[ Folder/Title.md#section|alias ]] [[Title]] ![[image.png]] ![[Sub/Note.md]] [[File.pdf]]\n" +
		"`[[inline]]` ``[[double]] ` code`` [[Visible]]\n```md\n[[fenced]]\n````\n" +
		"~~~\n[[tilde]]\n~~~\n`multiline\n[[hidden]]` [[Final]] \\[[escaped]]"
	want := []string{"Folder/Title", "Title", "Sub/Note", "Visible", "Final"}
	if got := links(body); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestJournal(t *testing.T) {
	body := "# Journal\r\n- 2026-10-20 - [[AI/Claude Code|Claude]] - text [[Project]] - more  \r\n" +
		"- 2026-02-30 - [[AI]] - invalid\n- other line\n"
	want := []model.JournalEntry{{Date: "2026-10-20", AI: "Claude Code", Text: "text [[Project]] - more  "}}
	if got := journal(body); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestReadNoteBOMAndLists(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Sessions"), 0700); err != nil {
		t.Fatal(err)
	}
	raw := "\ufeff---\r\ntype: session\r\ndate: 2026-10-26\r\nproject: ['[[Folder/Atlas|A]]', '[[Other]]']\r\nai: ['[[Claude Code|Claude]]']\r\n---\r\n# Body\r\n[[Atlas]]\r\n"
	file := filepath.Join(root, "Sessions", "test.md")
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sessions) != 1 || r.Sessions[0].Project != "Atlas" || r.Sessions[0].AI != "Claude Code" || r.Notes[0].Content != "# Body\r\n[[Atlas]]\r\n" {
		t.Fatalf("result: %#v", r)
	}
	if _, err := ReadNote(root, file); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside.md", "other.txt"} {
		if _, err := ReadNote(root, name); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
}

func TestFrontMatter(t *testing.T) {
	for _, text := range []string{"", "# No YAML\r\n", "---\ntype: project\n"} {
		body, meta := frontMatter(text)
		if body != text || len(meta) != 0 {
			t.Fatalf("unexpected parse of %q", text)
		}
	}
	for _, header := range []string{"type: [broken", "- list", "type: project\ntype: session"} {
		body, meta := frontMatter("---\n" + header + "\n---\nbody")
		if body != "body" || len(meta) != 0 {
			t.Fatalf("invalid YAML not tolerated: %q", header)
		}
	}
}

func TestAbsolutePath(t *testing.T) {
	for _, value := range []string{"C:/Fixtures/Test", `D:\Test`, "/tmp/test"} {
		if !absolutePath(value) {
			t.Fatalf("rejected %q", value)
		}
	}
	for _, value := range []string{"", "none (not created yet)", "C:relative", "relative/path"} {
		if absolutePath(value) {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestHiddenDirectoriesAndNonMarkdown(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".obsidian", ".drafts", ".trash", "Visible/.hidden"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "cache.md"), []byte("hidden"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "keep.md"), []byte("kept"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignore.txt"), []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Read(root)
	if err != nil || len(r.Notes) != 1 || r.Notes[0].Title != "keep" {
		t.Fatalf("result: %#v, %v", r, err)
	}
	if _, err := Read(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing root accepted")
	}
}

func TestDecomposedFilename(t *testing.T) {
	root := t.TempDir()
	nfd := "Nai\u0308ve.md"
	if err := os.WriteFile(filepath.Join(root, nfd), []byte("# Hi\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Notes) != 1 {
		t.Fatalf("notes: %#v", r.Notes)
	}
	got := r.Notes[0]
	wantPath, wantTitle := nfd, strings.TrimSuffix(nfd, ".md")
	if runtime.GOOS == "darwin" {
		wantPath, wantTitle = "Na\u00efve.md", "Na\u00efve"
	}
	if got.Path != wantPath || got.Title != wantTitle {
		t.Fatalf("path %q title %q", got.Path, got.Title)
	}
	if _, err := ReadNote(root, got.Path); err != nil {
		t.Fatal(err)
	}
}

func TestHiddenFileAndMarkdownCase(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".secret.md"), []byte("# Hidden\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Upper.MD"), []byte("# Upper\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Notes) != 1 || r.Notes[0].Title != "Upper" {
		t.Fatalf("%#v", r.Notes)
	}
}

func TestStarJournalLine(t *testing.T) {
	body := "* 2026-10-20 - [[AI/Claude Code]] - kept\n- 2026-02-30 - [[AI]] - invalid\nnot a journal line\n"
	got := journal(body)
	if len(got) != 1 || got[0].Text != "kept" || got[0].AI != "Claude Code" {
		t.Fatalf("%#v", got)
	}
}

func TestLeadingSHA(t *testing.T) {
	for input, want := range map[string]string{
		"0df64eb2 (official, unmodified)": "0df64eb2",
		"E5843FE":                         "e5843fe",
		"none":                            "",
		"abc":                             "",
		"deadbeefz":                       "",
		" 0f085ad ":                       "0f085ad",
		"deadbeef-stale":                  "",
		"deadbeef (stale)":                "deadbeef",
	} {
		if got := leadingSHA(input); got != want {
			t.Errorf("leadingSHA(%q) = %q, want %q", input, got, want)
		}
	}
}

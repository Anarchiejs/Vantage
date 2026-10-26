package vault

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/model"
)

func TestLireFixture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "coffre")
	r, err := Lire(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Notes) != 14 || len(r.Fiches) != 2 || len(r.Sessions) != 2 || len(r.Journal) != 2 {
		t.Fatalf("counts notes/fiches/sessions/journal = %d/%d/%d/%d", len(r.Notes), len(r.Fiches), len(r.Sessions), len(r.Journal))
	}
	want := []model.Fiche{
		{Nom: "Atlas", Statut: "actif", Maj: "2026-10-26", VerifieLe: "2026-10-26", VerifieCommit: "a1b2c3d", Technos: []string{"Go", "TypeScript"}},
		{Nom: "Boussole", Statut: "archive", CheminSource: "C:/Fixtures/Boussole", Maj: "2026-07-01", VerifieLe: "2026-07-01", VerifieCommit: "deadbeef-perime", Technos: []string{"Markdown"}},
	}
	if !reflect.DeepEqual(r.Fiches, want) {
		t.Fatalf("fiches: %#v", r.Fiches)
	}
	for _, s := range r.Sessions {
		if s.IA != "Claude Code" || (s.Projet != "Atlas" && s.Projet != "Boussole") || !strings.HasPrefix(s.CheminNote, "Sessions/") || s.Date == "" {
			t.Fatalf("session: %#v", s)
		}
	}
	if r.Journal[0] != (model.EntreeJournal{Date: "2026-10-26", IA: "Claude Code", Texte: "préparation des fixtures fictives avec [[Atlas]]."}) {
		t.Fatalf("journal: %#v", r.Journal)
	}
	foundOrphan, foundInvalid := false, false
	for _, n := range r.Notes {
		if strings.Contains(n.Chemin, "\\") {
			t.Fatalf("path: %s", n.Chemin)
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.Chemin)))
		if err != nil {
			t.Fatal(err)
		}
		if n.Hash != fmt.Sprintf("%x", sha256.Sum256(raw)) {
			t.Fatal("incorrect raw hash")
		}
		single, err := LireNote(root, n.Chemin)
		if err != nil || !reflect.DeepEqual(n, single) {
			t.Fatalf("LireNote mismatch: %v", err)
		}
		switch n.Titre {
		case "Échantillonnage":
			foundOrphan = true
		case "Brouillon":
			foundInvalid = true
			if n.Type != "" || n.Maj != "" || !strings.HasPrefix(n.Contenu, "# Brouillon") {
				t.Fatalf("invalid YAML: %#v", n)
			}
		case "Atlas":
			links := []string{"Atlas - Resume", "Atlas - Detaille", "2026-10-20 - Atlas - import initial", "Cartographie", "Note absente", "Note"}
			if !reflect.DeepEqual(n.Liens, links) || n.Dossier != "Projets" {
				t.Fatalf("Atlas: %#v", n)
			}
		}
	}
	if !foundOrphan || !foundInvalid {
		t.Fatal("orphan or invalid note lost")
	}
}

func TestLiens(t *testing.T) {
	body := "[[ Dossier/Titre.md#section|alias ]] [[Titre]] ![[image.png]] ![[Sous/Note.md]] [[Fichier.pdf]]\n" +
		"`[[inline]]` ``[[double]] ` code`` [[Visible]]\n```md\n[[fenced]]\n````\n" +
		"~~~\n[[tilde]]\n~~~\n`multiline\n[[hidden]]` [[Final]] \\[[escaped]]"
	want := []string{"Titre", "Note", "Visible", "Final"}
	if got := liens(body); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestJournal(t *testing.T) {
	body := "# Journal\r\n- 2026-10-20 - [[IA/Claude Code|Claude]] - texte [[Projet]] - suite  \r\n" +
		"- 2026-02-30 - [[IA]] - invalide\n- autre ligne\n"
	want := []model.EntreeJournal{{Date: "2026-10-20", IA: "Claude Code", Texte: "texte [[Projet]] - suite  "}}
	if got := journal(body); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestLireNoteBOMAndLists(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Sessions"), 0700); err != nil {
		t.Fatal(err)
	}
	raw := "\ufeff---\r\ntype: session\r\ndate: 2026-10-26\r\nprojet: ['[[Dossier/Atlas|A]]', '[[Autre]]']\r\nia: ['[[Claude Code|Claude]]']\r\n---\r\n# Corps\r\n[[Atlas]]\r\n"
	file := filepath.Join(root, "Sessions", "test.md")
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Lire(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sessions) != 1 || r.Sessions[0].Projet != "Atlas" || r.Sessions[0].IA != "Claude Code" || r.Notes[0].Contenu != "# Corps\r\n[[Atlas]]\r\n" {
		t.Fatalf("result: %#v", r)
	}
	if _, err := LireNote(root, file); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside.md", "other.txt"} {
		if _, err := LireNote(root, name); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
}

func TestFrontMatter(t *testing.T) {
	for _, text := range []string{"", "# Sans YAML\r\n", "---\ntype: projet\n"} {
		body, meta := frontMatter(text)
		if body != text || len(meta) != 0 {
			t.Fatalf("unexpected parse of %q", text)
		}
	}
	for _, header := range []string{"type: [broken", "- list", "type: projet\ntype: session"} {
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
	for _, value := range []string{"", "aucun (pas encore créé)", "C:relative", "relative/path"} {
		if absolutePath(value) {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestHiddenDirectoriesAndNonMarkdown(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".obsidian", ".cerveau", ".trash", "Visible/.hidden"} {
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
	r, err := Lire(root)
	if err != nil || len(r.Notes) != 1 || r.Notes[0].Titre != "keep" {
		t.Fatalf("result: %#v, %v", r, err)
	}
	if _, err := Lire(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing root accepted")
	}
}

package report

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/db"
	"github.com/Anarchiejs/Observatoire/internal/ingest"
	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
)

var maj = flag.Bool("update", false, "rewrite the reviewed Markdown expectation")

const dateRapport = "2026-03-21" // week of March 16-21: 2 commits per repository in the fixtures

func racineProjet() string {
	_, fichier, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(fichier), "../.."))
}

func baseFixture(t *testing.T) *sql.DB {
	t.Helper()
	racine := racineProjet()
	if _, err := os.Stat(filepath.Join(racine, "testdata/depots/alpha/.git")); err != nil {
		t.Fatalf("Git fixtures are missing (do not regenerate them automatically): %v", err)
	}
	b, err := db.Open(filepath.Join(t.TempDir(), "report.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	if err := db.Migrate(b); err != nil {
		t.Fatal(err)
	}
	resultat, err := ingest.Import(context.Background(), b, ingest.Options{
		Vault: filepath.Join(racine, "testdata/vault"),
		Repos: map[string]string{"Atlas": filepath.Join(racine, "testdata/depots/alpha"), "Compass": filepath.Join(racine, "testdata/depots/beta")},
		Today: dateRapport,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resultat.Commits != 42 || len(resultat.Warnings) != 0 {
		t.Fatalf("import: %+v", resultat)
	}
	return b
}

func TestGenererAttendu(t *testing.T) {
	b := baseFixture(t)
	contenu, err := Generate(context.Background(), gen.New(b), dateRapport)
	if err != nil {
		t.Fatal(err)
	}
	chemin := filepath.Join("testdata", "attendu.md")
	if *maj {
		if err := os.MkdirAll(filepath.Dir(chemin), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(chemin, contenu, 0644); err != nil {
			t.Fatal(err)
		}
	}
	attendu, err := os.ReadFile(chemin)
	if err != nil {
		t.Fatal(err)
	}
	if string(contenu) != string(attendu) {
		t.Fatalf("report differs from %s; review before -update", chemin)
	}
}

func TestEcrireProtegeNoteEtrangere(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Statistics.md")
	original := []byte("---\ntype: note\n---\nManual content\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, []byte("new")); err == nil {
		t.Fatal("Write replaced a note without the owner marker")
	}
	actuel, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actuel) != string(original) {
		t.Fatal("the manual note was modified")
	}
}

func TestEcrireRemplaceEtNettoieTemporaire(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Statistics.md")
	if err := os.WriteFile(path, []byte("---\ngenerated_by: observatoire\n---\nold\n"), 0644); err != nil {
		t.Fatal(err)
	}
	new := []byte("---\ngenerated_by: observatoire\n---\nnew\n")
	if err := Write(dir, new); err != nil {
		t.Fatal(err)
	}
	actuel, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actuel) != string(new) {
		t.Fatal("generated content was not replaced")
	}
	entrees, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entrees) != 1 || entrees[0].Name() != "Statistics.md" {
		t.Fatalf("temporary file left behind: %v", entrees)
	}
}

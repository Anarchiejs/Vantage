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

var maj = flag.Bool("maj", false, "rewrite the reviewed Markdown expectation")

const dateRapport = "2026-03-21" // semaine du 16 au 21 mars : 2 commits par dépôt dans les fixtures

func racineProjet() string {
	_, fichier, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(fichier), "../.."))
}

func baseFixture(t *testing.T) *sql.DB {
	t.Helper()
	racine := racineProjet()
	if _, err := os.Stat(filepath.Join(racine, "testdata/depots/alpha/.git")); err != nil {
		t.Fatalf("fixtures Git absentes (ne pas les regenerer automatiquement): %v", err)
	}
	b, err := db.Ouvrir(filepath.Join(t.TempDir(), "report.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	if err := db.Migrer(b); err != nil {
		t.Fatal(err)
	}
	resultat, err := ingest.Importer(context.Background(), b, ingest.Options{
		Coffre:     filepath.Join(racine, "testdata/coffre"),
		Depots:     map[string]string{"Atlas": filepath.Join(racine, "testdata/depots/alpha"), "Boussole": filepath.Join(racine, "testdata/depots/beta")},
		Aujourdhui: dateRapport,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resultat.Commits != 42 || len(resultat.Avertissements) != 0 {
		t.Fatalf("import: %+v", resultat)
	}
	return b
}

func TestGenererAttendu(t *testing.T) {
	b := baseFixture(t)
	contenu, err := Generer(context.Background(), gen.New(b), dateRapport)
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
		t.Fatalf("rapport différent de %s; relire avant -maj", chemin)
	}
}

func TestEcrireProtegeNoteEtrangere(t *testing.T) {
	coffre := t.TempDir()
	path := filepath.Join(coffre, "Statistiques.md")
	original := []byte("---\ntype: note\n---\nContenu manuel\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Ecrire(coffre, []byte("nouveau")); err == nil {
		t.Fatal("Ecrire a remplace une note sans marqueur propriétaire")
	}
	actuel, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actuel) != string(original) {
		t.Fatal("la note manuelle a été modifiée")
	}
}

func TestEcrireRemplaceEtNettoieTemporaire(t *testing.T) {
	coffre := t.TempDir()
	path := filepath.Join(coffre, "Statistiques.md")
	if err := os.WriteFile(path, []byte("---\ngenere_par: observatoire\n---\nancien\n"), 0644); err != nil {
		t.Fatal(err)
	}
	nouveau := []byte("---\ngenere_par: observatoire\n---\nnouveau\n")
	if err := Ecrire(coffre, nouveau); err != nil {
		t.Fatal(err)
	}
	actuel, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actuel) != string(nouveau) {
		t.Fatal("contenu généré non remplacé")
	}
	entrees, err := os.ReadDir(coffre)
	if err != nil {
		t.Fatal(err)
	}
	if len(entrees) != 1 || entrees[0].Name() != "Statistiques.md" {
		t.Fatalf("fichier temporaire restant: %v", entrees)
	}
}

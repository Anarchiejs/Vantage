package ingest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/db"
)

// racine renvoie la racine du dépôt Observatoire.
func racine(t *testing.T) string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func preparer(t *testing.T) (*sql.DB, Options) {
	t.Helper()
	r := racine(t)
	// Les dépôts sont créés une fois avant `go test ./...` (voir testdata/README.md) : les régénérer ici
	// casserait les tests des autres packages qui tournent en même temps.
	if _, err := os.Stat(filepath.Join(r, "testdata", "depots", "alpha", ".git")); err != nil {
		t.Fatal("fixtures absentes : lancer d'abord go run ./scripts/fixtures/make-repos.go")
	}
	base, err := db.Ouvrir(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { base.Close() })
	if err := db.Migrer(base); err != nil {
		t.Fatal(err)
	}
	return base, Options{
		Coffre:     filepath.Join(r, "testdata", "coffre"),
		Depots:     map[string]string{"Atlas": filepath.Join(r, "testdata", "depots", "alpha"), "Boussole": filepath.Join(r, "testdata", "depots", "beta")},
		Aujourdhui: "2026-10-26",
	}
}

func compter(t *testing.T, base *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := base.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("%s : %v", q, err)
	}
	return n
}

func TestImportIdempotent(t *testing.T) {
	base, opt := preparer(t)
	ctx := context.Background()

	b1, err := Importer(ctx, base, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(b1.Avertissements) > 0 {
		t.Fatalf("avertissements : %v", b1.Avertissements)
	}
	if b1.Depots != 2 || b1.Commits != 42 {
		t.Fatalf("premier import : %+v", b1)
	}

	tables := []string{"projet", "depot", "commit_git", "fichier_modifie", "note", "lien", "session", "journal", "techno", "projet_techno", "mesure_langage", "recherche", "activite_jour"}
	avant := map[string]int{}
	for _, tb := range tables {
		avant[tb] = compter(t, base, "SELECT count(*) FROM "+tb)
	}
	totalAvant := compter(t, base, "SELECT sum(nb_commits) FROM activite_jour")

	b2, err := Importer(ctx, base, opt)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Commits != 0 || b2.NotesEcrites != 0 {
		t.Fatalf("deuxième import non vide : %+v", b2)
	}
	for _, tb := range tables {
		if n := compter(t, base, "SELECT count(*) FROM "+tb); n != avant[tb] {
			t.Errorf("%s : %d puis %d lignes", tb, avant[tb], n)
		}
	}
	if totalAvant != 42 {
		t.Errorf("activite_jour compte %d commits, 42 attendus", totalAvant)
	}
	if n := compter(t, base, "SELECT count(*) FROM lien WHERE cible_id IS NULL"); n == 0 {
		t.Error("aucun lien cassé trouvé alors que les fixtures en contiennent")
	}
	if n := compter(t, base, "SELECT count(*) FROM recherche WHERE recherche MATCH 'echantillonnage'"); n == 0 {
		t.Error("recherche sans accents : rien trouvé")
	}
}

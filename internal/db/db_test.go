package db

import (
	"path/filepath"
	"testing"
)

func TestMigrerEtTriggers(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "test.db")
	base, err := Ouvrir(chemin)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	if err := Migrer(base); err != nil {
		t.Fatal(err)
	}
	// Deuxième passage : rien à faire, pas d'erreur.
	if err := Migrer(base); err != nil {
		t.Fatal(err)
	}

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := base.Exec(q, args...); err != nil {
			t.Fatalf("%s : %v", q, err)
		}
	}
	exec(`INSERT INTO projet (id, nom) VALUES (1, 'Alpha')`)
	exec(`INSERT INTO depot (id, projet_id, chemin) VALUES (1, 1, 'C:/alpha')`)
	// 23:30 UTC à +120 min = le lendemain 01:30 en heure locale.
	exec(`INSERT INTO commit_git (sha, depot_id, auteur, date_utc, decalage_min, message)
	      VALUES ('a1', 1, 'moi', '2026-10-25T23:30:00Z', 120, 'premier')`)
	exec(`INSERT INTO fichier_modifie VALUES ('a1', 'main.go', 'Go', 10, 2)`)
	exec(`INSERT INTO fichier_modifie VALUES ('a1', 'logo.png', NULL, NULL, NULL)`)

	var jour string
	var heure, n, aj, sup int
	if err := base.QueryRow(`SELECT jour_local, heure_locale FROM commit_git WHERE sha = 'a1'`).Scan(&jour, &heure); err != nil {
		t.Fatal(err)
	}
	if jour != "2026-10-26" || heure != 1 {
		t.Fatalf("colonnes calculées : jour=%s heure=%d", jour, heure)
	}
	if err := base.QueryRow(`SELECT nb_commits, ajouts, suppressions FROM activite_jour WHERE jour = '2026-10-26' AND projet_id = 1`).Scan(&n, &aj, &sup); err != nil {
		t.Fatal(err)
	}
	if n != 1 || aj != 10 || sup != 2 {
		t.Fatalf("activite_jour : %d %d %d", n, aj, sup)
	}

	exec(`DELETE FROM commit_git WHERE sha = 'a1'`)
	if err := base.QueryRow(`SELECT nb_commits, ajouts, suppressions FROM activite_jour WHERE jour = '2026-10-26'`).Scan(&n, &aj, &sup); err != nil {
		t.Fatal(err)
	}
	if n != 0 || aj != 0 || sup != 0 {
		t.Fatalf("après suppression : %d %d %d", n, aj, sup)
	}

	// Recherche plein texte sans accents.
	exec(`INSERT INTO recherche (titre, contenu, source, ref) VALUES ('Résumé', 'fenêtrage et CTE', 'note', 'x.md')`)
	if err := base.QueryRow(`SELECT count(*) FROM recherche WHERE recherche MATCH 'fenetrage'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("FTS5 sans accents : %d", n)
	}
}

func TestLectureSeule(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "test.db")
	rw, err := Ouvrir(chemin)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrer(rw); err != nil {
		t.Fatal(err)
	}
	rw.Close()

	ro, err := OuvrirLectureSeule(chemin)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err := ro.Exec(`INSERT INTO projet (nom) VALUES ('x')`); err == nil {
		t.Fatal("une écriture a réussi en lecture seule")
	}
	if _, err := ro.Exec(`DROP TABLE projet`); err == nil {
		t.Fatal("DROP a réussi en lecture seule")
	}
}

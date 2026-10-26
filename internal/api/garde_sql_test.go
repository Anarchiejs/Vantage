package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/db"
)

func sqlConsole(h http.Handler, requete string) (int, string) {
	corps, _ := json.Marshal(DemandeSQL{SQL: requete})
	w := appeler(h, "POST", "/api/sql", "127.0.0.1", string(corps))
	return w.Code, w.Body.String()
}

// Contournements trouvés par la revue de sécurité : un commentaire en tête suffisait à passer la regex.
func TestConsoleAuthorizer(t *testing.T) {
	_, h := serveurTest(t)
	autre := filepath.Join(t.TempDir(), "autre.db")
	b, err := db.Ouvrir(autre)
	if err != nil {
		t.Fatal(err)
	}
	b.Exec(`CREATE TABLE secret (x TEXT); INSERT INTO secret VALUES ('confidentiel')`)
	b.Close()
	uri := "file:" + filepath.ToSlash(autre) + "?mode=ro"

	for _, requete := range []string{
		fmt.Sprintf(`/* x */ ATTACH DATABASE '%s' AS ext; SELECT * FROM ext.secret`, uri),
		fmt.Sprintf(`SELECT 1; ATTACH '%s' AS ext`, uri),
		`/*x*/ PRAGMA query_only=0`,
		`-- commentaire
PRAGMA query_only(0)`,
		`PRAGMA journal_mode = DELETE`,
		`PRAGMA wal_checkpoint`,
		`BEGIN; SELECT count(*) FROM projet`,
		`SAVEPOINT a`,
		`CREATE TEMP TABLE t (x)`,
		`SELECT zeroblob(100000000)`,
	} {
		if code, corps := sqlConsole(h, requete); code == http.StatusOK {
			t.Errorf("accepté : %s → %s", requete, corps)
		}
	}
	for _, requete := range []string{
		`PRAGMA table_info(projet)`,
		`SELECT name FROM pragma_table_xinfo('projet')`,
		`PRAGMA user_version`,
		`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 3) SELECT i FROM n`,
		`SELECT nom FROM projet WHERE nom LIKE '%a%'`,
	} {
		if code, corps := sqlConsole(h, requete); code != http.StatusOK {
			t.Errorf("refusé : %s → %d %s", requete, code, corps)
		}
	}
	// Le schéma utilise les mêmes connexions : il doit rester lisible une fois l'authorizer installé.
	if w := appeler(h, "GET", "/api/schema", "127.0.0.1", ""); w.Code != http.StatusOK {
		t.Fatalf("schéma : %d %s", w.Code, w.Body)
	}
}

// La première ligne arrive tout de suite ; la seconde lance un calcul sans fin. Le délai doit
// l'interrompre aussi pendant le parcours des lignes.
func TestConsoleDelaiPendantLesLignes(t *testing.T) {
	ancien := delaiConsole
	delaiConsole = 300 * time.Millisecond
	t.Cleanup(func() { delaiConsole = ancien })
	_, h := serveurTest(t)

	fini := make(chan int, 1)
	go func() {
		code, _ := sqlConsole(h, `WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n)
SELECT 0 UNION ALL SELECT sum(x) FROM n`)
		fini <- code
	}()
	select {
	case code := <-fini:
		if code == http.StatusOK {
			t.Fatal("requête infinie terminée avec succès")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("requête non interrompue pendant le parcours des lignes")
	}
	// La connexion interrompue est réutilisable.
	if code, corps := sqlConsole(h, `SELECT count(*) FROM projet`); code != http.StatusOK {
		t.Fatalf("après interruption : %d %s", code, corps)
	}
}

// Graphe dense (clique de 40 notes) et destination isolée : l'ancienne récursion SQL sur les chemins
// simples ne terminait pas.
func TestCheminGrapheDense(t *testing.T) {
	s, h := serveurTest(t)
	base := s.cfg.Base
	for i := 1; i <= 41; i++ {
		if _, err := base.Exec(`INSERT INTO note (id, chemin, dossier, titre, contenu_hash) VALUES (?, ?, 'Concepts', ?, 'h')`,
			i, fmt.Sprintf("Concepts/N%d.md", i), fmt.Sprintf("N%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 40; i++ {
		for j := 1; j <= 40; j++ {
			if i != j {
				base.Exec(`INSERT INTO lien (source_id, cible_titre, cible_id) VALUES (?, ?, ?)`, i, fmt.Sprintf("N%d", j), j)
			}
		}
	}
	debut := time.Now()
	ch := lire[Chemin](t, h, "/api/graphe/chemin?de=1&a=41", 200)
	if len(ch.Chemin) != 0 {
		t.Fatalf("chemin vers une note isolée : %+v", ch.Chemin)
	}
	ch = lire[Chemin](t, h, "/api/graphe/chemin?de=1&a=40", 200)
	if len(ch.Chemin) != 2 || ch.Chemin[0].ID != 1 || ch.Chemin[1].ID != 40 {
		t.Fatalf("chemin direct : %+v", ch.Chemin)
	}
	if d := time.Since(debut); d > 2*time.Second {
		t.Fatalf("trop lent : %s", d)
	}
	lire[Erreur](t, h, "/api/graphe/chemin?de=1&a=999", 404)
	ch = lire[Chemin](t, h, "/api/graphe/chemin?de=5&a=5", 200)
	if len(ch.Chemin) != 1 {
		t.Fatalf("chemin vers soi : %+v", ch.Chemin)
	}
}

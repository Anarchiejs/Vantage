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
	corps, _ := json.Marshal(SQLRequest{SQL: requete})
	w := appeler(h, "POST", "/api/sql", "127.0.0.1", string(corps))
	return w.Code, w.Body.String()
}

// Bypasses found by the security review: a leading comment used to be enough to pass the regex.
func TestConsoleAuthorizer(t *testing.T) {
	_, h := serveurTest(t)
	autre := filepath.Join(t.TempDir(), "autre.db")
	b, err := db.Open(autre)
	if err != nil {
		t.Fatal(err)
	}
	b.Exec(`CREATE TABLE secret (x TEXT); INSERT INTO secret VALUES ('secret')`)
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
		`BEGIN; SELECT count(*) FROM project`,
		`SAVEPOINT a`,
		`CREATE TEMP TABLE t (x)`,
		`SELECT zeroblob(100000000)`,
	} {
		if code, corps := sqlConsole(h, requete); code == http.StatusOK {
			t.Errorf("accepted: %s → %s", requete, corps)
		}
	}
	for _, requete := range []string{
		`PRAGMA table_info(project)`,
		`SELECT name FROM pragma_table_xinfo('project')`,
		`PRAGMA user_version`,
		`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 3) SELECT i FROM n`,
		`SELECT name FROM project WHERE name LIKE '%a%'`,
	} {
		if code, corps := sqlConsole(h, requete); code != http.StatusOK {
			t.Errorf("refused: %s → %d %s", requete, code, corps)
		}
	}
	// The schema uses the same connections: it must stay readable once the authorizer is installed.
	if w := appeler(h, "GET", "/api/schema", "127.0.0.1", ""); w.Code != http.StatusOK {
		t.Fatalf("schema: %d %s", w.Code, w.Body)
	}
}

// The first row arrives immediately; the second starts an endless calculation. The deadline must
// interrupt it while scanning rows too.
func TestConsoleDelayPendantLesLignes(t *testing.T) {
	ancien := consoleTimeout
	consoleTimeout = 300 * time.Millisecond
	t.Cleanup(func() { consoleTimeout = ancien })
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
			t.Fatal("endless query finished successfully")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("query was not interrupted while scanning rows")
	}
	// The interrupted connection can be reused.
	if code, corps := sqlConsole(h, `SELECT count(*) FROM project`); code != http.StatusOK {
		t.Fatalf("after interruption : %d %s", code, corps)
	}
}

// A dense graph (a clique of 40 notes) and an isolated destination: the old SQL recursion over simple
// paths did not finish.
func TestCheminGrapheDense(t *testing.T) {
	s, h := serveurTest(t)
	base := s.cfg.DB
	for i := 1; i <= 41; i++ {
		if _, err := base.Exec(`INSERT INTO note (id, path, folder, title, content_hash) VALUES (?, ?, 'Concepts', ?, 'h')`,
			i, fmt.Sprintf("Concepts/N%d.md", i), fmt.Sprintf("N%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 40; i++ {
		for j := 1; j <= 40; j++ {
			if i != j {
				base.Exec(`INSERT INTO link (source_id, target_title, target_id) VALUES (?, ?, ?)`, i, fmt.Sprintf("N%d", j), j)
			}
		}
	}
	debut := time.Now()
	ch := lire[GraphPath](t, h, "/api/graph/path?from=1&to=41", 200)
	if len(ch.Path) != 0 {
		t.Fatalf("path to an isolated note : %+v", ch.Path)
	}
	ch = lire[GraphPath](t, h, "/api/graph/path?from=1&to=40", 200)
	if len(ch.Path) != 2 || ch.Path[0].ID != 1 || ch.Path[1].ID != 40 {
		t.Fatalf("direct path : %+v", ch.Path)
	}
	if d := time.Since(debut); d > 2*time.Second {
		t.Fatalf("too slow : %s", d)
	}
	lire[APIError](t, h, "/api/graph/path?from=1&to=999", 404)
	ch = lire[GraphPath](t, h, "/api/graph/path?from=5&to=5", 200)
	if len(ch.Path) != 1 {
		t.Fatalf("path to itself : %+v", ch.Path)
	}
}

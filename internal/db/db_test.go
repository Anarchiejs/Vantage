package db

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateAndTriggers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	base, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	if err := Migrate(base); err != nil {
		t.Fatal(err)
	}
	// A second pass has nothing to do and returns no error.
	if err := Migrate(base); err != nil {
		t.Fatal(err)
	}

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := base.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO project (id, name) VALUES (1, 'Alpha')`)
	exec(`INSERT INTO repository (id, project_id, path) VALUES (1, 1, 'C:/alpha')`)
	// 23:30 UTC at +120 min is 01:30 the next local day.
	exec(`INSERT INTO commit_git (sha, repository_id, author, date_utc, offset_min, message)
	      VALUES ('a1', 1, 'me', '2026-10-25T23:30:00Z', 120, 'first')`)
	exec(`INSERT INTO changed_file (repository_id, commit_sha, path, language, additions, deletions) VALUES (1, 'a1', 'main.go', 'Go', 10, 2)`)
	exec(`INSERT INTO changed_file (repository_id, commit_sha, path, language, additions, deletions) VALUES (1, 'a1', 'logo.png', NULL, NULL, NULL)`)

	var day string
	var hour, n, additions, deletions int
	if err := base.QueryRow(`SELECT local_day, local_hour FROM commit_git WHERE sha = 'a1'`).Scan(&day, &hour); err != nil {
		t.Fatal(err)
	}
	if day != "2026-10-26" || hour != 1 {
		t.Fatalf("generated columns: day=%s hour=%d", day, hour)
	}
	if err := base.QueryRow(`SELECT commit_count, additions, deletions FROM daily_activity WHERE day = '2026-10-26' AND project_id = 1`).Scan(&n, &additions, &deletions); err != nil {
		t.Fatal(err)
	}
	if n != 1 || additions != 10 || deletions != 2 {
		t.Fatalf("daily_activity: %d %d %d", n, additions, deletions)
	}

	exec(`DELETE FROM commit_git WHERE sha = 'a1'`)
	if err := base.QueryRow(`SELECT commit_count, additions, deletions FROM daily_activity WHERE day = '2026-10-26'`).Scan(&n, &additions, &deletions); err != nil {
		t.Fatal(err)
	}
	if n != 0 || additions != 0 || deletions != 0 {
		t.Fatalf("after delete: %d %d %d", n, additions, deletions)
	}

	// Full-text search folds accents.
	exec(`INSERT INTO search_index (title, content, source, ref) VALUES ('Naïve', 'windowing and CTE', 'note', 'x.md')`)
	if err := base.QueryRow(`SELECT count(*) FROM search_index WHERE search_index MATCH 'naive'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("FTS5 without accents: %d", n)
	}
}

func TestRefuseLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	base, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	if _, err := base.Exec(`CREATE TABLE projet (id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	err = Migrate(base)
	if !errors.Is(err, ErrLegacySchema) {
		t.Fatalf("legacy schema: %v", err)
	}
}

func TestRetiredDatabaseName(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "observatoire.db")
	if err := os.WriteFile(legacy, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(legacy); !errors.Is(err, ErrRetiredDatabase) {
		t.Fatalf("opening observatoire.db: %v", err)
	}
	if _, err := Open(filepath.Join(dir, "vantage.db")); !errors.Is(err, ErrRetiredDatabase) {
		t.Fatalf("missing vantage.db beside observatoire.db: %v", err)
	}
}

func TestDSNEscapesPath(t *testing.T) {
	// Space and '#' must be escaped in the SQLite DSN. '?' is reserved in a Windows path.
	dir := filepath.Join(t.TempDir(), "a b#hash")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vantage.db")
	base, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	if err := Migrate(base); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Exec(`SELECT 1`); err != nil {
		t.Fatal(err)
	}
}

func TestReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	rw, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(rw); err != nil {
		t.Fatal(err)
	}
	rw.Close()

	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err := ro.Exec(`INSERT INTO project (name) VALUES ('x')`); err == nil {
		t.Fatal("a write succeeded in read-only mode")
	}
	if _, err := ro.Exec(`DROP TABLE project`); err == nil {
		t.Fatal("DROP succeeded in read-only mode")
	}
}

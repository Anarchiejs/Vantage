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

// root returns the Observatoire repository root.
func root(t *testing.T) string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func prepare(t *testing.T) (*sql.DB, Options) {
	t.Helper()
	r := root(t)
	// The repositories are created once before `go test ./...` (see testdata/README.md): regenerating them here
	// would break the other packages' tests running at the same time.
	if _, err := os.Stat(filepath.Join(r, "testdata", "repos", "alpha", ".git")); err != nil {
		t.Fatal("fixtures missing: run go run ./scripts/fixtures/make-repos.go first")
	}
	base, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { base.Close() })
	if err := db.Migrate(base); err != nil {
		t.Fatal(err)
	}
	return base, Options{
		Vault: filepath.Join(r, "testdata", "vault"),
		Repos: map[string]string{"Atlas": filepath.Join(r, "testdata", "repos", "alpha"), "Compass": filepath.Join(r, "testdata", "repos", "beta")},
		Today: "2026-10-26",
	}
}

func countRows(t *testing.T, base *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := base.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("%s : %v", q, err)
	}
	return n
}

func TestImportIdempotent(t *testing.T) {
	base, opt := prepare(t)
	ctx := context.Background()

	b1, err := Import(ctx, base, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(b1.Warnings) > 0 {
		t.Fatalf("warnings: %v", b1.Warnings)
	}
	if b1.Repos != 2 || b1.Commits != 42 {
		t.Fatalf("first import: %+v", b1)
	}

	tables := []string{"project", "repository", "commit_git", "changed_file", "note", "link", "session", "journal", "technology", "project_technology", "language_stat", "search_index", "daily_activity"}
	before := map[string]int{}
	for _, tb := range tables {
		before[tb] = countRows(t, base, "SELECT count(*) FROM "+tb)
	}
	totalBefore := countRows(t, base, "SELECT sum(commit_count) FROM daily_activity")

	b2, err := Import(ctx, base, opt)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Commits != 0 || b2.NotesWritten != 0 {
		t.Fatalf("second import was not empty : %+v", b2)
	}
	for _, tb := range tables {
		if n := countRows(t, base, "SELECT count(*) FROM "+tb); n != before[tb] {
			t.Errorf("%s: %d then %d rows", tb, before[tb], n)
		}
	}
	if totalBefore != 42 {
		t.Errorf("daily_activity has %d commits, expected 42", totalBefore)
	}
	if n := countRows(t, base, "SELECT count(*) FROM link WHERE target_id IS NULL"); n == 0 {
		t.Error("no broken link found, but the fixtures contain some")
	}
	if n := countRows(t, base, "SELECT count(*) FROM search_index WHERE search_index MATCH 'naive'"); n == 0 {
		t.Error("accent-insensitive search found nothing")
	}
}

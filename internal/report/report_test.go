package report

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Anarchiejs/Vantage/internal/db"
	"github.com/Anarchiejs/Vantage/internal/ingest"
	"github.com/Anarchiejs/Vantage/internal/sql/gen"
)

var maj = flag.Bool("update", false, "rewrite the reviewed Markdown expectation")

const reportDate = "2026-03-21" // week of March 16-21: 2 commits per repository in the fixtures

func projectRoot() string {
	_, callerFile, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(callerFile), "../.."))
}

func baseFixture(t *testing.T) *sql.DB {
	t.Helper()
	root := projectRoot()
	b, err := db.Open(filepath.Join(t.TempDir(), "report.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	if err := db.Migrate(b); err != nil {
		t.Fatal(err)
	}
	result, err := ingest.Import(context.Background(), b, ingest.Options{
		Vault: filepath.Join(root, "testdata/vault"),
		Repos: map[string]string{"Atlas": filepath.Join(root, "testdata/repos/alpha"), "Compass": filepath.Join(root, "testdata/repos/beta")},
		Today: reportDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Commits != 42 || len(result.Warnings) != 0 {
		t.Fatalf("import: %+v", result)
	}
	return b
}

func TestGeneratedReport(t *testing.T) {
	b := baseFixture(t)
	content, err := Generate(context.Background(), gen.New(b), reportDate)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "expected.md")
	if *maj {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(expected) {
		t.Fatalf("report differs from %s; review before -update", path)
	}
}

func TestWriteRefusesForeignNote(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Statistics.md")
	original := []byte("---\ntype: note\n---\nManual content\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, []byte("new")); err == nil {
		t.Fatal("Write replaced a note without the owner marker")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(original) {
		t.Fatal("the manual note was modified")
	}
}

func TestWriteReplacesAndRemovesTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Statistics.md")
	if err := os.WriteFile(path, []byte("---\ngenerated_by: observatoire\n---\nold\n"), 0644); err != nil {
		t.Fatal(err)
	}
	new := []byte("---\ngenerated_by: observatoire\n---\nnew\n")
	if err := Write(dir, new); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(new) {
		t.Fatal("generated content was not replaced")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "Statistics.md" {
		t.Fatalf("temporary file left behind: %v", entries)
	}
}

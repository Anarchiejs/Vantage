package ingest

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Anarchiejs/Vantage/internal/db"
)

func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func makeRepo(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "init", "-b", "main")
	gitAt(t, dir, "config", "user.email", "dev@example.com")
	gitAt(t, dir, "config", "user.name", "Dev")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "add", "main.go")
	gitAt(t, dir, "commit", "-m", "first")
	return gitAt(t, dir, "rev-parse", "HEAD")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestReimportMatchesTheVault(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	makeRepo(t, alpha)
	if err := os.Remove(filepath.Join(alpha, "main.go")); err != nil {
		t.Fatal(err)
	}
	beta := filepath.Join(root, "beta")
	gitAt(t, root, "clone", alpha, beta)
	vault := filepath.Join(root, "vault")
	writeFile(t, filepath.Join(vault, "Projects", "Alpha", "Alpha.md"), "---\ntype: project\nstatus: active\nsource_path: "+alpha+"\ntechnologies:\n  - \"[[go]]\"\n---\n# Alpha\n[[Go]]\n")
	writeFile(t, filepath.Join(vault, "Projects", "Beta", "Beta.md"), "---\ntype: project\nstatus: active\nsource_path: "+beta+"\n---\n# Beta\n")
	writeFile(t, filepath.Join(vault, "Go.md"), "# Go\n")
	writeFile(t, filepath.Join(vault, "Journal.md"), "- 2026-10-20 - [[Go]] - first\n- 2026-10-21 - [[Go]] - second\n")
	writeFile(t, filepath.Join(vault, "Sessions", "2026-10-20 - Alpha - start.md"), "---\ntype: session\ndate: 2026-10-20\nproject: \"[[alpha]]\"\nai: \"[[go]]\"\n---\n# Start\n")

	base, err := db.Open(filepath.Join(root, "vantage.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { base.Close() })
	if err := db.Migrate(base); err != nil {
		t.Fatal(err)
	}
	summary, err := Import(context.Background(), base, Options{Vault: vault, Today: "2026-10-26"})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Commits < 2 {
		t.Fatalf("shared history should land in both clones: %+v", summary)
	}
	var commits, shas, repos, techs, journal, sessions int
	mustScan(t, base, `SELECT count(*) FROM commit_git`, &commits)
	mustScan(t, base, `SELECT count(DISTINCT sha) FROM commit_git`, &shas)
	mustScan(t, base, `SELECT count(*) FROM repository`, &repos)
	if commits != 2 || shas != 1 || repos != 2 {
		t.Fatalf("commits=%d shas=%d repos=%d", commits, shas, repos)
	}
	mustScan(t, base, `SELECT count(*) FROM technology`, &techs)
	if techs != 1 {
		t.Fatalf("technology name was not canonicalized: %d", techs)
	}
	var target sql.NullInt64
	if err := base.QueryRow(`SELECT target_id FROM link WHERE target_title = 'go'`).Scan(&target); err != nil || !target.Valid {
		t.Fatalf("[[go]] did not resolve: %v %v", target, err)
	}

	writeFile(t, filepath.Join(vault, "Journal.md"), "- 2026-10-22 - [[Go]] - only\n")
	writeFile(t, filepath.Join(vault, "Sessions", "2026-10-20 - Alpha - start.md"), "---\ntype: note\n---\n# Start\n")
	os.Remove(filepath.Join(vault, "Projects", "Beta", "Beta.md"))
	writeFile(t, filepath.Join(vault, "Projects", "Alpha", "Alpha.md"), "---\ntype: project\nstatus: active\nsource_path: "+alpha+"\n---\n# Alpha\n")
	if _, err := Import(context.Background(), base, Options{Vault: vault, Today: "2026-10-26"}); err != nil {
		t.Fatal(err)
	}
	mustScan(t, base, `SELECT count(*) FROM project WHERE name = 'Beta'`, &repos)
	mustScan(t, base, `SELECT count(*) FROM journal`, &journal)
	mustScan(t, base, `SELECT count(*) FROM session`, &sessions)
	mustScan(t, base, `SELECT count(*) FROM technology`, &techs)
	mustScan(t, base, `SELECT count(*) FROM repository`, &commits)
	if repos != 0 || journal != 1 || sessions != 0 || techs != 0 || commits != 1 {
		t.Fatalf("after reimport projects=%d journal=%d sessions=%d techs=%d repos=%d", repos, journal, sessions, techs, commits)
	}
}

func mustScan(t *testing.T, base *sql.DB, query string, dest *int) {
	t.Helper()
	if err := base.QueryRow(query).Scan(dest); err != nil {
		t.Fatal(err)
	}
}

package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFixtures(t *testing.T) {
	for _, nom := range []string{"alpha", "beta"} {
		t.Run(nom, func(t *testing.T) {
			depot := filepath.Join("..", "..", "..", "testdata", "depots", nom)
			ctx := context.Background()
			commits, err := Read(ctx, depot, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(commits) != 21 {
				t.Fatalf("commits = %d, attendu 21", len(commits))
			}
			sha, branche, err := Head(ctx, depot)
			if err != nil || branche != "main" || sha != commits[20].SHA {
				t.Fatalf("Tete = %s %s %v", sha, branche, err)
			}
			var rename, binaire, fusion, accent, corps bool
			positions := make(map[string]int)
			for i, c := range commits {
				positions[c.SHA] = i
				if c.IsMerge {
					fusion = true
					if len(c.Files) != 0 {
						t.Fatal("fichiers de fusion")
					}
				}
				if strings.Contains(c.Message, "First body line.\nSecond body line.") {
					corps = true
				}
				for _, f := range c.Files {
					switch f.Path {
					case "docs/manual.md":
						rename = true
						if f.Additions == nil || *f.Additions != 0 || *f.Deletions != 0 {
							t.Fatalf("renommage: %+v", f)
						}
					case "assets/sample.bin":
						binaire = true
						if f.Additions != nil || f.Deletions != nil {
							t.Fatal("binary counted as text")
						}
					case "docs/note é.md":
						accent = true
					}
				}
			}
			if !rename || !binaire || !fusion || !accent || !corps {
				t.Fatalf("rename=%v binaire=%v fusion=%v accent=%v corps=%v", rename, binaire, fusion, accent, corps)
			}
			// Dates volontairement non monotones : l'ordre doit rester topologique.
			parents, err := output(ctx, depot, "rev-list", "--parents", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			for _, ligne := range strings.Split(parents, "\n") {
				champs := strings.Fields(ligne)
				for _, p := range champs[1:] {
					if positions[p] >= positions[champs[0]] {
						t.Fatal("parent after child")
					}
				}
			}
			for i, offset := range []int64{120, 120, 120, -300, -300, 330, 120} {
				if commits[i].OffsetMin() != offset {
					t.Fatalf("fuseau %d = %d", i, commits[i].OffsetMin())
				}
			}
			if commits[6].DateUTC() != "2026-01-13T23:30:00Z" {
				t.Fatal(commits[6].DateUTC())
			}
			suite, err := Read(ctx, depot, commits[9].SHA)
			if err != nil || len(suite) != 11 {
				t.Fatalf("incremental: %d %v", len(suite), err)
			}
			for i, c := range suite {
				if c.SHA != commits[i+10].SHA {
					t.Fatal("suite incorrecte")
				}
			}
			vide, err := Read(ctx, depot, sha)
			if err != nil || len(vide) != 0 {
				t.Fatalf("up to date: %d %v", len(vide), err)
			}
			tout, err := Read(ctx, depot, strings.Repeat("f", 40))
			if !errors.Is(err, ErrHistoryRewritten) || len(tout) != 21 {
				t.Fatalf("SHA inconnu: %d %v", len(tout), err)
			}
		})
	}
}

func TestParseur(t *testing.T) {
	flux := "\x00abc\x00parent\x00Author é\x002026-01-14T01:30:00+02:00\x00Subject\n\nBody\nsecond line\n\x00\x00\n2\t1\tdocs/note é.md\x000\t0\t\x00old\nname.ts\x00new\t é.ts\x00-\t-\timage.bin\x00\x00def\x00abc parent2\x00Other\x002026-01-14T00:00:00Z\x00Merge\n\x00\x00"
	commits, err := readStream(strings.NewReader(flux))
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatal(len(commits))
	}
	c := commits[0]
	if c.Message != "Subject\n\nBody\nsecond line" || c.Author != "Author é" || c.OffsetMin() != 120 {
		t.Fatalf("commit: %+v", c)
	}
	if len(c.Files) != 3 {
		t.Fatal(c.Files)
	}
	if f := c.Files[0]; f.Path != "docs/note é.md" || f.Language != "Markdown" || *f.Additions != 2 || *f.Deletions != 1 {
		t.Fatal(f)
	}
	if f := c.Files[1]; f.Path != "new\t é.ts" || f.Language != "TypeScript" {
		t.Fatal(f)
	}
	if c.Files[2].Additions != nil || !commits[1].IsMerge {
		t.Fatal(commits)
	}
	for _, invalide := range []string{"truncated", "\x00sha\x00", "\x00s\x00\x00a\x00date\x00m\x00", "1\t2\tf\x00", "\x00s\x00\x00a\x002026-01-01T00:00:00Z\x00m\x00\nx\t2\tf\x00"} {
		if _, err := readStream(strings.NewReader(invalide)); err == nil {
			t.Fatalf("invalid stream accepted: %q", invalide)
		}
	}
}

func TestLangages(t *testing.T) {
	for ext, attendu := range languages {
		if obtenu := language("dir/fichier" + strings.ToUpper(ext)); obtenu != attendu {
			t.Errorf("%s: %s", ext, obtenu)
		}
	}
	if language("sans-extension") != "" || language("image.png") != "" {
		t.Fatal("extension inconnue")
	}
}

func TestHistoriqueReecritEtTeteDetachee(t *testing.T) {
	depot := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		s, err := output(context.Background(), depot, args...)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	run("init", "-q", "-b", "main")
	commit := func(msg string) {
		run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", msg)
	}
	commit("root")
	run("checkout", "-qb", "old")
	commit("old")
	oldSHA := run("rev-parse", "HEAD")
	run("checkout", "-q", "main")
	commit("new")
	commits, err := Read(context.Background(), depot, oldSHA)
	if !errors.Is(err, ErrHistoryRewritten) || len(commits) != 2 {
		t.Fatalf("rewrite: %d %v", len(commits), err)
	}
	run("checkout", "-q", "--detach")
	sha, branche, err := Head(context.Background(), depot)
	if err != nil || sha == "" || branche != "" {
		t.Fatalf("detached HEAD: %s %s %v", sha, branche, err)
	}
}

func TestErreurs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, ".", ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := Head(ctx, "."); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), t.TempDir(), ""); err == nil || errors.Is(err, ErrHistoryRewritten) {
		t.Fatal(err)
	}
}

// Mesure le chemin complet (processus Git inclus) sans seuil temporel fragile
// dans les tests ordinaires. Le dépôt synthétique reste dans le dossier temporaire.
func BenchmarkLire10000(b *testing.B) {
	dir := b.TempDir()
	cmd := exec.Command("git", "init", "-q", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("%v: %s", err, out)
	}
	var flux strings.Builder
	for i := 1; i <= 10000; i++ {
		fmt.Fprintf(&flux, "commit refs/heads/main\nmark :%d\nauthor Test <test@example.invalid> 1760000000 +0200\ncommitter Test <test@example.invalid> 1760000000 +0200\ndata 7\ncommit\n", i)
		if i > 1 {
			fmt.Fprintf(&flux, "from :%d\n", i-1)
		}
		contenu := fmt.Sprintf("line %d\n", i)
		fmt.Fprintf(&flux, "M 100644 inline file.go\ndata %d\n%s", len(contenu), contenu)
		flux.WriteByte('\n')
	}
	cmd = exec.Command("git", "-C", dir, "fast-import", "--quiet")
	cmd.Stdin = strings.NewReader(flux.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("%v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		b.Fatalf("%v: %s", err, out)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		cs, err := Read(context.Background(), dir, "")
		if err != nil || len(cs) != 10000 {
			b.Fatalf("%d: %v", len(cs), err)
		}
		b.ReportMetric(time.Since(start).Seconds(), "s/import")
	}
}

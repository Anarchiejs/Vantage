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
	for _, name := range []string{"alpha", "beta"} {
		t.Run(name, func(t *testing.T) {
			repo := filepath.Join("..", "..", "..", "testdata", "repos", name)
			ctx := context.Background()
			commits, err := Read(ctx, repo, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(commits) != 21 {
				t.Fatalf("commits = %d, expected 21", len(commits))
			}
			sha, branch, err := Head(ctx, repo)
			if err != nil || branch != "main" || sha != commits[20].SHA {
				t.Fatalf("Head = %s %s %v", sha, branch, err)
			}
			var rename, binaryFile, mergeCommit, accent, messageBody bool
			positions := make(map[string]int)
			for i, c := range commits {
				positions[c.SHA] = i
				if c.IsMerge {
					mergeCommit = true
					if len(c.Files) != 0 {
						t.Fatal("fichiers de mergeCommit")
					}
				}
				if strings.Contains(c.Message, "First body line.\nSecond body line.") {
					messageBody = true
				}
				for _, f := range c.Files {
					switch f.Path {
					case "docs/manual.md":
						rename = true
						if f.Additions == nil || *f.Additions != 0 || *f.Deletions != 0 {
							t.Fatalf("renommage: %+v", f)
						}
					case "assets/sample.bin":
						binaryFile = true
						if f.Additions != nil || f.Deletions != nil {
							t.Fatal("binary counted as text")
						}
					case "docs/note é.md":
						accent = true
					}
				}
			}
			if !rename || !binaryFile || !mergeCommit || !accent || !messageBody {
				t.Fatalf("rename=%v binaryFile=%v mergeCommit=%v accent=%v messageBody=%v", rename, binaryFile, mergeCommit, accent, messageBody)
			}
			// Dates volontairement non monotones : l'ordre doit rester topologique.
			parents, err := output(ctx, repo, "rev-list", "--parents", "HEAD")
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
			rest, err := Read(ctx, repo, commits[9].SHA)
			if err != nil || len(rest) != 11 {
				t.Fatalf("incremental: %d %v", len(rest), err)
			}
			for i, c := range rest {
				if c.SHA != commits[i+10].SHA {
					t.Fatal("wrong order")
				}
			}
			empty, err := Read(ctx, repo, sha)
			if err != nil || len(empty) != 0 {
				t.Fatalf("up to date: %d %v", len(empty), err)
			}
			allCommits, err := Read(ctx, repo, strings.Repeat("f", 40))
			if !errors.Is(err, ErrHistoryRewritten) || len(allCommits) != 21 {
				t.Fatalf("unknown SHA: %d %v", len(allCommits), err)
			}
		})
	}
}

func TestParser(t *testing.T) {
	stream := "\x00abc\x00parent\x00Author é\x002026-01-14T01:30:00+02:00\x00Subject\n\nBody\nsecond line\n\x00\x00\n2\t1\tdocs/note é.md\x000\t0\t\x00old\nname.ts\x00new\t é.ts\x00-\t-\timage.bin\x00\x00def\x00abc parent2\x00Other\x002026-01-14T00:00:00Z\x00Merge\n\x00\x00"
	commits, err := readStream(strings.NewReader(stream))
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
	for _, invalid := range []string{"truncated", "\x00sha\x00", "\x00s\x00\x00a\x00date\x00m\x00", "1\t2\tf\x00", "\x00s\x00\x00a\x002026-01-01T00:00:00Z\x00m\x00\nx\t2\tf\x00"} {
		if _, err := readStream(strings.NewReader(invalid)); err == nil {
			t.Fatalf("invalid stream accepted: %q", invalid)
		}
	}
}

func TestLanguages(t *testing.T) {
	for ext, want := range languages {
		if got := language("dir/fileName" + strings.ToUpper(ext)); got != want {
			t.Errorf("%s: %s", ext, got)
		}
	}
	if language("no-extension") != "" || language("image.png") != "" {
		t.Fatal("unknown extension")
	}
}

func TestRewrittenHistoryAndDetachedHead(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		s, err := output(context.Background(), repo, args...)
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
	commits, err := Read(context.Background(), repo, oldSHA)
	if !errors.Is(err, ErrHistoryRewritten) || len(commits) != 2 {
		t.Fatalf("rewrite: %d %v", len(commits), err)
	}
	run("checkout", "-q", "--detach")
	sha, branch, err := Head(context.Background(), repo)
	if err != nil || sha == "" || branch != "" {
		t.Fatalf("detached HEAD: %s %s %v", sha, branch, err)
	}
}

func TestErrors(t *testing.T) {
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

// Times the full path, including the Git process, without a fragile time limit
// in ordinary tests. The synthetic repository stays in the temporary directory.
func BenchmarkRead10000(b *testing.B) {
	dir := b.TempDir()
	cmd := exec.Command("git", "init", "-q", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("%v: %s", err, out)
	}
	var stream strings.Builder
	for i := 1; i <= 10000; i++ {
		fmt.Fprintf(&stream, "commit refs/heads/main\nmark :%d\nauthor Test <test@example.invalid> 1760000000 +0200\ncommitter Test <test@example.invalid> 1760000000 +0200\ndata 7\ncommit\n", i)
		if i > 1 {
			fmt.Fprintf(&stream, "from :%d\n", i-1)
		}
		contents := fmt.Sprintf("line %d\n", i)
		fmt.Fprintf(&stream, "M 100644 inline file.go\ndata %d\n%s", len(contents), contents)
		stream.WriteByte('\n')
	}
	cmd = exec.Command("git", "-C", dir, "fast-import", "--quiet")
	cmd.Stdin = strings.NewReader(stream.String())
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

// Package fixtures builds the deterministic Git repositories under testdata/repos.
// Tests call Ensure. The command in scripts/fixtures rebuilds them from scratch.
package fixtures

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Ensure creates testdata/repos when alpha or beta is missing.
// Concurrent calls, including go test ./..., share one lock and do not rebuild
// repositories that are already present.
func Ensure() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	return withLock(root, func() error {
		if ready(root) {
			return nil
		}
		return rebuild(root)
	})
}

// Build deletes testdata/repos and creates alpha and beta again.
// root is the repository root (the directory that contains go.mod).
func Build(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	return withLock(root, func() error { return rebuild(root) })
}

func moduleRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate the fixtures package")
	}
	root, err := ModuleRoot(filepath.Dir(file))
	if err != nil {
		return "", err
	}
	return filepath.Abs(root)
}

// ModuleRoot walks up from start and returns the directory that contains go.mod.
func ModuleRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", start)
		}
		dir = parent
	}
}

func ready(root string) bool {
	for _, name := range []string{"alpha", "beta"} {
		if _, err := os.Stat(filepath.Join(root, "testdata", "repos", name, ".git")); err != nil {
			return false
		}
	}
	return true
}

func rebuild(root string) error {
	dir := filepath.Join(root, "testdata", "repos")
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for _, name := range []string{"alpha", "beta"} {
		if err := buildRepo(filepath.Join(dir, name), name); err != nil {
			return err
		}
	}
	return nil
}

func withLock(root string, fn func() error) error {
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	path := filepath.Join(os.TempDir(), fmt.Sprintf("observatoire-fixtures-%x.lock", sum[:8]))
	if err := acquire(path, filepath.Join(root, "testdata", "repos")); err != nil {
		return err
	}
	defer os.Remove(path)
	return fn()
}

func acquire(path, repos string) error {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return nil
		}
		if !os.IsExist(err) {
			return err
		}
		info, statErr := os.Stat(path)
		if statErr == nil && time.Since(info.ModTime()) > 3*time.Minute {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting to generate %s", repos)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

type change struct {
	name    string
	data    []byte
	remove  string
	branch  bool
	merge   bool
	date    string
	author  string
	message string
}

func buildRepo(dir, project string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := git(dir, nil, "init", "-q", "-b", "main"); err != nil {
		return err
	}
	if err := git(dir, nil, "config", "user.name", "Fixture Author"); err != nil {
		return err
	}
	if err := git(dir, nil, "config", "user.email", "fixture@example.invalid"); err != nil {
		return err
	}
	// Pin settings that otherwise follow the machine, so fixture hashes match
	// on Linux, macOS, and Windows. core.precomposeunicode is a no-op outside macOS.
	for _, kv := range [][2]string{
		{"commit.gpgsign", "false"},
		{"core.autocrlf", "false"},
		{"core.filemode", "false"},
		{"core.precomposeunicode", "true"},
	} {
		if err := git(dir, nil, "config", kv[0], kv[1]); err != nil {
			return err
		}
	}
	files := map[string][]byte{}
	for i := 0; i < 20; i++ {
		date := fmt.Sprintf("2026-%02d-%02dT12:00:00Z", 1+i/7, 5+(i%7)*3)
		c := change{name: fmt.Sprintf("notes/%s-%02d.md", project, i+1), data: []byte(fmt.Sprintf("# %s fixture %02d\n\nSynthetic history entry.\n", project, i+1)), date: date, author: "Fixture Author"}
		switch i {
		case 1:
			c.name = "main.go"
			c.data = []byte("package main\n\nfunc main() {}\n")
		case 2:
			c.name = "web/app.ts"
			c.data = []byte("export const fixture = true;\n")
		case 3:
			c.name = "db/schema.sql"
			c.data = []byte("CREATE TABLE fixture (id INTEGER PRIMARY KEY);\n")
		case 4:
			c.name = "README.md"
			c.data = []byte("# Synthetic repository\n")
		case 6:
			c.name = "assets/sample.bin"
			c.data = []byte{0, 1, 2, 255, 0, 127}
		case 8:
			c.remove = "docs/guide.md"
			c.name = "docs/manual.md"
			c.data = []byte("# Fixture guide\n\n" + strings.Repeat("Stable documentation line.\n", 10))
		case 9:
			c.name = "docs/note é.md"
			c.data = []byte("# Accented note\n\nPath with a space and an accent.\n")
		case 12:
			c.branch = true
			c.name = "branch.txt"
			c.data = []byte("side branch\n")
		case 13:
			c.merge = true
		case 14:
			c.message = "fixture " + project + " multiline subject\n\nFirst body line.\nSecond body line."
		}
		// Seed a stable guide before its later rename.
		if i == 7 {
			p := filepath.Join(dir, "docs", "guide.md")
			if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(p, []byte("# Fixture guide\n\n"+strings.Repeat("Stable documentation line.\n", 10)), 0644); err != nil {
				return err
			}
		}
		// Six explicit ISO timestamps with offsets, including 23:30 UTC represented
		// as the following local calendar day at +02:00.
		switch i {
		case 0:
			c.date = "2026-01-05T14:00:00+02:00"
		case 1:
			c.date = "2026-01-08T14:00:00+02:00"
		case 2:
			c.date = "2026-01-11T14:00:00+02:00"
		case 3:
			c.date = "2026-01-07T18:30:00-05:00"
		case 4:
			c.date = "2026-01-10T18:30:00-05:00"
		case 5:
			c.date = "2026-01-12T05:00:00+05:30"
		case 6:
			c.date = "2026-01-14T01:30:00+02:00" // 2026-01-13 23:30 UTC
		}
		if i == 10 || i == 16 {
			c.author = "Second Fixture"
		}
		if i == 19 && project == "beta" {
			c.author = "Third Fixture"
		}
		if c.branch {
			if err := git(dir, env(c.date, c.author), "checkout", "-q", "-b", "fixture-side"); err != nil {
				return err
			}
		}
		if c.remove != "" {
			if err := os.Remove(filepath.Join(dir, filepath.FromSlash(c.remove))); err != nil {
				return err
			}
			delete(files, c.remove)
		}
		if c.name != "" {
			p := filepath.Join(dir, filepath.FromSlash(c.name))
			if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(p, c.data, 0644); err != nil {
				return err
			}
			files[c.name] = c.data
		}
		if err := git(dir, env(c.date, c.author), "add", "-A"); err != nil {
			return err
		}
		msg := c.message
		if msg == "" {
			msg = fmt.Sprintf("fixture %s change %02d", project, i+1)
		}
		if c.merge {
			if err := git(dir, env(c.date, c.author), "checkout", "-q", "main"); err != nil {
				return err
			}
			// Advance main after the side branch, then merge its independent file.
			p := filepath.Join(dir, "mainline.txt")
			if err := os.WriteFile(p, []byte("main line\n"), 0644); err != nil {
				return err
			}
			if err := git(dir, env(c.date, c.author), "add", "-A"); err != nil {
				return err
			}
			if err := commit(dir, c.date, c.author, msg+" mainline"); err != nil {
				return err
			}
			if err := git(dir, env(c.date, c.author), "merge", "--no-ff", "-m", msg+" merge", "fixture-side"); err != nil {
				return err
			}
			if err := git(dir, nil, "branch", "-D", "fixture-side"); err != nil {
				return err
			}
			continue
		}
		if err := commit(dir, c.date, c.author, msg); err != nil {
			return err
		}
	}
	return nil
}

func env(date, author string) []string {
	return []string{"GIT_AUTHOR_NAME=" + author, "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=" + author, "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
}

func commit(dir, date, author, msg string) error {
	return git(dir, env(date, author), "commit", "-q", "-m", msg)
}

func git(dir string, extra []string, args ...string) error {
	// Ignore commit.gpgsign from the ambient Git config so commit hashes stay stable in CI.
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for _, item := range extra {
		split := 0
		for split < len(item) && item[split] != '=' {
			split++
		}
		k := item[:split]
		filtered := cmd.Env[:0]
		for _, old := range cmd.Env {
			if len(old) > len(k) && old[:len(k)+1] == k+"=" {
				continue
			}
			filtered = append(filtered, old)
		}
		cmd.Env = append(filtered, item)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w: %s", args, err, out.String())
	}
	return nil
}

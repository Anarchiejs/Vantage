package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testDelay = 100 * time.Millisecond

type rig struct {
	t            *testing.T
	root         string
	cancel       context.CancelFunc
	fini         chan error
	imports      chan int
	n            atomic.Int32
	mu           sync.Mutex
	repoPaths    []string
	handleImport func(context.Context, int) (Changes, error)
	announceFn   func(...string)
	logs         chan string
}

func newRig(t *testing.T, repos ...string) *rig {
	t.Helper()
	b := &rig{t: t, root: t.TempDir(), fini: make(chan error, 1), imports: make(chan int, 128), repoPaths: repos, logs: make(chan string, 128)}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	opt := Options{
		Vault: b.root, Delay: testDelay,
		Repos: func() []string { b.mu.Lock(); defer b.mu.Unlock(); return append([]string(nil), b.repoPaths...) },
		Import: func(ctx context.Context) (Changes, error) {
			n := int(b.n.Add(1))
			b.imports <- n
			b.mu.Lock()
			f := b.handleImport
			b.mu.Unlock()
			if f != nil {
				return f(ctx, n)
			}
			return Changes{}, nil
		},
		Announce: func(tables ...string) {
			b.mu.Lock()
			f := b.announceFn
			b.mu.Unlock()
			if f != nil {
				f(tables...)
			}
		},
		Log: func(format string, args ...any) {
			select {
			case b.logs <- format:
			default:
			}
		},
	}
	go func() { b.fini <- Watch(ctx, opt) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-b.fini:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("cancellation blocked")
		}
	})
	// A repeated probe waits until the watches are actually installed.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ready := false
	for !ready {
		select {
		case <-ticker.C:
			b.writeNote(filepath.Join(b.root, "probe.md"), "ready")
		case <-b.imports:
			ready = true
		case <-deadline.C:
			t.Fatal("watch not ready")
		}
	}
	b.settle(3 * testDelay)
	b.n.Store(0)
	return b
}

func (b *rig) writeNote(path, contents string) {
	b.t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		b.t.Fatal(err)
	}
}

func (b *rig) waitForImport(n int) {
	b.t.Helper()
	select {
	case got := <-b.imports:
		if got != n {
			b.t.Fatalf("import %d, expected %d", got, n)
		}
	case <-time.After(3 * time.Second):
		b.t.Fatalf("import %d absent", n)
	}
}

func (b *rig) settle(d time.Duration) {
	b.t.Helper()
	// Startup can still produce an import that is already queued.
	timer := time.NewTimer(d)
	defer timer.Stop()
	limitTimer := time.NewTimer(3 * time.Second)
	defer limitTimer.Stop()
	for {
		select {
		case <-b.imports:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(d)
		case <-timer.C:
			return
		case <-limitTimer.C:
			b.t.Fatal("startup did not settle")
		}
	}
}

func (b *rig) expectQuiet() {
	b.t.Helper()
	select {
	case n := <-b.imports:
		b.t.Fatalf("unexpected import : %d", n)
	case <-time.After(3 * testDelay):
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
}

func TestChanges(t *testing.T) {
	notes := []string{"note", "link", "session", "journal", "search_index"}
	commits := []string{"commit_git", "changed_file", "daily_activity", "language_stat"}
	for _, tc := range []struct {
		c      Changes
		tables []string
	}{
		{Changes{}, nil}, {Changes{Notes: 1}, notes}, {Changes{Commits: 1}, commits},
		{Changes{Notes: 2, Commits: 3}, append(append([]string{}, notes...), commits...)},
	} {
		if got := tc.c.Tables(); !reflect.DeepEqual(got, tc.tables) {
			t.Fatalf("Tables : %v", got)
		}
		if tc.c.Empty() != (len(tc.tables) == 0) {
			t.Fatal("Empty is inconsistent")
		}
	}
}

func TestNotesAndBatching(t *testing.T) {
	b := newRig(t)
	b.writeNote(filepath.Join(b.root, "note.md"), "a note")
	b.waitForImport(1)
	b.expectQuiet()
	for i := 0; i < 20; i++ {
		b.writeNote(filepath.Join(b.root, "note.md"), "modification")
	}
	b.waitForImport(2)
	b.expectQuiet()
}

func TestIgnoreAndNewFolders(t *testing.T) {
	b := newRig(t)
	for _, dir := range []string{".obsidian", ".trash", ".git", ".cache"} {
		mkdir(t, filepath.Join(b.root, dir))
		b.writeNote(filepath.Join(b.root, dir, "cache.md"), "ignored")
	}
	b.writeNote(filepath.Join(b.root, ".obsidian", "x.json"), "{}")
	b.writeNote(filepath.Join(b.root, "note.txt"), "ignored")
	b.expectQuiet()
	sub := filepath.Join(b.root, "fresh", "nested")
	mkdir(t, sub)
	b.writeNote(filepath.Join(sub, "note.md"), "new note")
	b.waitForImport(1)
	b.expectQuiet()
	b.writeNote(filepath.Join(sub, "note.md"), "later edit")
	b.waitForImport(2)
	b.expectQuiet()
}

func TestGit(t *testing.T) {
	repoDir := t.TempDir()
	git := filepath.Join(repoDir, ".git")
	mkdir(t, filepath.Join(git, "refs", "heads"))
	mkdir(t, filepath.Join(git, "objects"))
	b := newRig(t, repoDir)
	for _, name := range []string{"index.lock", "objects/objet", "refs/heads/main.lock"} {
		b.writeNote(filepath.Join(git, filepath.FromSlash(name)), "ignored")
	}
	b.expectQuiet()
	for i, name := range []string{"refs/heads/main", "HEAD", "packed-refs", "refs/heads/team/branch"} {
		path := filepath.Join(git, filepath.FromSlash(name))
		mkdir(t, filepath.Dir(path))
		b.writeNote(path, "new ref")
		b.waitForImport(i + 1)
		b.expectQuiet()
	}
}

func TestSlowImport(t *testing.T) {
	b := newRig(t)
	liberer := make(chan struct{})
	var activeCount, maximum atomic.Int32
	b.mu.Lock()
	b.handleImport = func(ctx context.Context, n int) (Changes, error) {
		a := activeCount.Add(1)
		defer activeCount.Add(-1)
		if a > maximum.Load() {
			maximum.Store(a)
		}
		if n == 1 {
			select {
			case <-liberer:
			case <-ctx.Done():
			}
		}
		return Changes{}, nil
	}
	b.mu.Unlock()
	b.writeNote(filepath.Join(b.root, "slow.md"), "first")
	b.waitForImport(1)
	for i := 0; i < 20; i++ {
		b.writeNote(filepath.Join(b.root, "during.md"), "during")
	}
	b.expectQuiet()
	close(liberer)
	b.waitForImport(2)
	b.expectQuiet()
	if maximum.Load() != 1 {
		t.Fatal("imports concurrents")
	}
}

func TestErrorAnnounceAndDynamicRepos(t *testing.T) {
	oldDir, newDir := t.TempDir(), t.TempDir()
	for _, repoDir := range []string{oldDir, newDir} {
		mkdir(t, filepath.Join(repoDir, ".git", "refs", "heads"))
	}
	b := newRig(t, oldDir)
	annonces := make(chan []string, 4)
	b.mu.Lock()
	b.announceFn = func(tables ...string) { annonces <- tables }
	b.handleImport = func(ctx context.Context, n int) (Changes, error) {
		if n == 1 {
			return Changes{Notes: 1}, errors.New("simulated failure")
		}
		if n == 2 {
			return Changes{Notes: 1, Commits: 1}, nil
		}
		return Changes{}, nil
	}
	b.repoPaths = []string{newDir}
	b.mu.Unlock()
	b.writeNote(filepath.Join(b.root, "note.md"), "failure")
	b.waitForImport(1)
	b.expectQuiet()
	select {
	case <-b.logs:
	case <-time.After(3 * time.Second):
		t.Fatal("error was not logged")
	}
	select {
	case <-annonces:
		t.Fatal("announcement after an error")
	default:
	}
	b.writeNote(filepath.Join(oldDir, ".git", "HEAD"), "oldDir")
	b.expectQuiet()
	b.writeNote(filepath.Join(newDir, ".git", "HEAD"), "newDir")
	b.waitForImport(2)
	select {
	case tables := <-annonces:
		if !reflect.DeepEqual(tables, (Changes{Notes: 1, Commits: 1}).Tables()) {
			t.Fatalf("annonce : %v", tables)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("annonce absente")
	}
	b.expectQuiet()
	b.writeNote(filepath.Join(b.root, "note.md"), "unchanged")
	b.waitForImport(3)
	b.expectQuiet()
	select {
	case <-annonces:
		t.Fatal("empty announcement")
	default:
	}
}

func TestGitdirAndCommonDir(t *testing.T) {
	repoDir, sharedDir := t.TempDir(), t.TempDir()
	git := filepath.Join(sharedDir, "worktrees", "test")
	mkdir(t, git)
	mkdir(t, filepath.Join(sharedDir, "refs", "heads"))
	if err := os.WriteFile(filepath.Join(repoDir, ".git"), []byte("gitdir: "+git+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git, "commondir"), []byte("../..\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b := newRig(t, repoDir)
	b.writeNote(filepath.Join(git, "HEAD"), "checkout")
	b.waitForImport(1)
	b.expectQuiet()
	b.writeNote(filepath.Join(sharedDir, "refs", "heads", "main"), "commit")
	b.waitForImport(2)
	b.expectQuiet()
}

func TestCancelAndSettle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Watch(ctx, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := Watch(context.Background(), Options{}); err == nil {
		t.Fatal("Importer requis")
	}
	if err := Watch(context.Background(), Options{Delay: -1, Import: func(context.Context) (Changes, error) { return Changes{}, nil }}); err == nil {
		t.Fatal("negative delay was accepted")
	}
	b := newRig(t)
	b.writeNote(filepath.Join(b.root, "pending.md"), "cancellation")
	b.cancel()
}

func TestRefCreationAndRelativeGitdir(t *testing.T) {
	repoDir := t.TempDir()
	git := filepath.Join(repoDir, "metadata")
	mkdir(t, git)
	if err := os.WriteFile(filepath.Join(repoDir, ".git"), []byte("gitdir: metadata\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b := newRig(t, repoDir)
	branchRef := filepath.Join(git, "refs", "heads", "main")
	mkdir(t, filepath.Dir(branchRef))
	b.writeNote(branchRef, "first commit")
	b.waitForImport(1)
	b.expectQuiet()
	b.writeNote(branchRef, "second commit")
	b.waitForImport(2)
	b.expectQuiet()
}

func TestCancelDuringImport(t *testing.T) {
	b := newRig(t)
	arrete := make(chan struct{})
	b.mu.Lock()
	b.handleImport = func(ctx context.Context, n int) (Changes, error) {
		<-ctx.Done()
		close(arrete)
		return Changes{}, ctx.Err()
	}
	b.mu.Unlock()
	b.writeNote(filepath.Join(b.root, "note.md"), "import blocked")
	b.waitForImport(1)
	b.cancel()
	select {
	case <-arrete:
	case <-time.After(3 * time.Second):
		t.Fatal("import was not canceled")
	}
}

func TestWindowFromFirstEvent(t *testing.T) {
	b := newRig(t)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	limitTimer := time.NewTimer(time.Second)
	defer limitTimer.Stop()
	for {
		select {
		case <-ticker.C:
			b.writeNote(filepath.Join(b.root, "continu.md"), "continuous writes")
		case <-b.imports:
			return
		case <-limitTimer.C:
			t.Fatal("window pushed back by later writes")
		}
	}
}

func TestRenameAndDeleteNotes(t *testing.T) {
	b := newRig(t)
	oldDir := filepath.Join(b.root, "oldDir.md")
	newDir := filepath.Join(b.root, "newDir.md")
	b.writeNote(oldDir, "note")
	b.waitForImport(1)
	b.expectQuiet()
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	b.waitForImport(2)
	b.expectQuiet()
	if err := os.Remove(newDir); err != nil {
		t.Fatal(err)
	}
	b.waitForImport(3)
	b.expectQuiet()
}

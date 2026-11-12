// Package watch watches notes and Git repository refs.
package watch

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/ospath"
	"github.com/fsnotify/fsnotify"
)

// Options defines the sources and callbacks for watching.
type Options struct {
	Vault    string
	Repos    func() []string
	Delay    time.Duration
	Import   func(ctx context.Context) (Changes, error)
	Announce func(tables ...string)
	Log      func(format string, args ...any)
}

// Changes counts the notes and commits modified by an import.
type Changes struct{ Notes, Commits int }

func (c Changes) Empty() bool { return c.Notes <= 0 && c.Commits <= 0 }

func (c Changes) Tables() []string {
	var tables []string
	if c.Notes > 0 {
		tables = append(tables, "note", "link", "session", "journal", "search_index")
	}
	if c.Commits > 0 {
		tables = append(tables, "commit_git", "changed_file", "daily_activity", "language_stat")
	}
	return tables
}

type watcher struct {
	w       *fsnotify.Watcher
	opt     Options
	vault   string
	git     []string
	watched map[string]bool
}

func (s *watcher) log(format string, args ...any) {
	if s.opt.Log != nil {
		s.opt.Log(format, args...)
	}
}

func normalize(path string) string {
	return ospath.ForCompare(path)
}

func absolute(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		return normalize(filepath.Clean(path))
	}
	// /tmp on macOS is a symlink to /private/tmp. Watch events use the
	// resolved path. Linux and Windows keep the path the caller passed.
	if runtime.GOOS == "darwin" {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			p = resolved
		}
	}
	return normalize(p)
}

func relativeTo(root, path string) (string, bool) {
	root = normalize(root)
	path = normalize(path)
	r, err := filepath.Rel(root, path)
	return r, err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

func visible(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") && part != "." {
			return false
		}
	}
	return true
}

func (s *watcher) walkTree(root string, wanted map[string]bool, cache bool) {
	root = normalize(root)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.log("walk %s: %v", path, err)
			return nil
		}
		path = normalize(path)
		if !d.IsDir() {
			return nil
		}
		if cache && path != root && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		wanted[path] = true
		// Watching before walking children closes the creation window.
		s.addWatch(path)
		return nil
	})
	if err != nil {
		s.log("watch %s: %v", root, err)
	}
}

func (s *watcher) addWatch(path string) {
	path = normalize(path)
	if s.watched[path] {
		return
	}
	if err := s.w.Add(path); err != nil {
		s.log("watch %s: %v", path, err)
	} else {
		s.watched[path] = true
	}
}

func (s *watcher) repos() {
	s.git = nil
	if s.opt.Repos == nil {
		return
	}
	for _, repo := range s.opt.Repos() {
		git := filepath.Join(absolute(repo), ".git")
		info, err := os.Stat(git)
		if err != nil {
			s.log("repository %s: %v", repo, err)
			continue
		}
		if !info.IsDir() {
			data, err := os.ReadFile(git)
			if err != nil {
				s.log("read %s: %v", git, err)
				continue
			}
			line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
			if !strings.HasPrefix(line, "gitdir:") {
				s.log("invalid gitdir file: %s", git)
				continue
			}
			dir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
			if dir == "" {
				s.log("empty gitdir file: %s", git)
				continue
			}
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(filepath.Dir(git), dir)
			}
			git = absolute(dir)
		}
		s.git = append(s.git, git)
		// Worktrees share branches in the common directory.
		if data, err := os.ReadFile(filepath.Join(git, "commondir")); err == nil {
			common := strings.TrimSpace(string(data))
			if common != "" {
				if !filepath.IsAbs(common) {
					common = filepath.Join(git, common)
				}
				s.git = append(s.git, absolute(common))
			}
		}
	}
}

func (s *watcher) refresh() {
	wanted := make(map[string]bool)
	if s.vault != "" {
		s.walkTree(s.vault, wanted, true)
	}
	for _, git := range s.git {
		wanted[git] = true
		s.addWatch(git)
		refs := filepath.Join(git, "refs")
		if info, err := os.Stat(refs); err == nil && info.IsDir() {
			wanted[refs] = true
			s.addWatch(refs)
		}
		heads := filepath.Join(refs, "heads")
		if _, err := os.Stat(heads); err == nil {
			s.walkTree(heads, wanted, false)
		}
	}
	for path := range s.watched {
		if !wanted[path] {
			_ = s.w.Remove(path)
			delete(s.watched, path)
		}
	}
	for path := range wanted {
		s.addWatch(path)
	}
}

func (s *watcher) relevant(event fsnotify.Event) bool {
	path := normalize(filepath.Clean(event.Name))
	info, statErr := os.Stat(path)
	directory := (statErr == nil && info.IsDir()) || s.watched[path]
	if directory && !event.Has(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) {
		// Windows also reports writes to the parent directory: only its contents matter.
		return false
	}
	if s.vault != "" {
		rel, ok := relativeTo(s.vault, path)
		parents := filepath.Dir(rel)
		if directory {
			parents = rel
		}
		if ok && visible(parents) {
			if statErr == nil && info.IsDir() {
				// A file can appear before the directory watch is installed.
				found := false
				_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if d.IsDir() && p != path && strings.HasPrefix(d.Name(), ".") {
						return filepath.SkipDir
					}
					if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".md") {
						found = true
					}
					return nil
				})
				return found
			}
			return strings.EqualFold(filepath.Ext(path), ".md") || (s.watched[path] && event.Has(fsnotify.Remove|fsnotify.Rename))
		}
	}
	for _, git := range s.git {
		if path == filepath.Join(git, "HEAD") || path == filepath.Join(git, "packed-refs") {
			return true
		}
		if path == filepath.Join(git, "refs") {
			if info, err := os.Stat(filepath.Join(path, "heads")); err == nil && info.IsDir() {
				return true
			}
		}
		if rel, ok := relativeTo(filepath.Join(git, "refs", "heads"), path); ok {
			return !strings.HasSuffix(rel, ".lock")
		}
	}
	return false
}

// Watch blocks until the context is canceled, without a concurrent import.
func Watch(ctx context.Context, opt Options) error {
	if ctx.Err() != nil {
		return nil
	}
	if opt.Import == nil {
		return fmt.Errorf("watch: Import is required")
	}
	if opt.Delay < 0 {
		return fmt.Errorf("watch: Delay must be positive")
	}
	if opt.Delay == 0 {
		opt.Delay = 1500 * time.Millisecond
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	s := &watcher{w: w, opt: opt, watched: make(map[string]bool)}
	if opt.Vault != "" {
		s.vault = absolute(opt.Vault)
	}
	s.repos()
	s.refresh()
	type outcome struct {
		c   Changes
		err error
	}
	fin := make(chan outcome, 1)
	var timer *time.Timer
	var attente <-chan time.Time
	enCours, suivant := false, false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	startImport := func() {
		enCours = true
		go func() { c, err := opt.Import(ctx); fin <- outcome{c, err} }()
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			// Deletes are still recognizable in the previous list.
			name := normalize(event.Name)
			removed := s.watched[name] && event.Has(fsnotify.Remove|fsnotify.Rename)
			relevant := false
			if removed {
				relevant = s.relevant(event)
				// fsnotify automatically drops deleted or renamed directories.
				for path := range s.watched {
					if _, ok := relativeTo(name, path); ok {
						_ = s.w.Remove(path)
						delete(s.watched, path)
					}
				}
			}
			info, statErr := os.Stat(name)
			if removed || (statErr == nil && info.IsDir() && event.Has(fsnotify.Create|fsnotify.Rename)) {
				s.refresh()
			}
			relevant = s.relevant(event) || relevant
			if !relevant {
				continue
			}
			if enCours {
				suivant = true
			} else if attente == nil {
				timer = time.NewTimer(opt.Delay)
				attente = timer.C
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			s.log("fsnotify error: %v", err)
		case <-attente:
			attente = nil
			if ctx.Err() == nil {
				startImport()
			}
		case result := <-fin:
			enCours = false
			if result.err != nil {
				s.log("import error: %v", result.err)
			} else if !result.c.Empty() && opt.Announce != nil {
				opt.Announce(result.c.Tables()...)
			}
			s.repos()
			s.refresh()
			if suivant && ctx.Err() == nil {
				suivant = false
				startImport()
			}
		}
	}
}

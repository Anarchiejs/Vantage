// Package watch surveille les notes et les références des dépôts Git.
package watch

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Options définit les sources et les rappels de la surveillance.
type Options struct {
	Coffre   string
	Depots   func() []string
	Delai    time.Duration
	Importer func(ctx context.Context) (Changements, error)
	Annoncer func(tables ...string)
	Journal  func(format string, args ...any)
}

// Changements compte les notes et les commits modifiés par un import.
type Changements struct{ Notes, Commits int }

func (c Changements) Vide() bool { return c.Notes <= 0 && c.Commits <= 0 }

func (c Changements) Tables() []string {
	var tables []string
	if c.Notes > 0 {
		tables = append(tables, "note", "lien", "session", "journal", "recherche")
	}
	if c.Commits > 0 {
		tables = append(tables, "commit_git", "fichier_modifie", "activite_jour", "mesure_langage")
	}
	return tables
}

type surveillance struct {
	w      *fsnotify.Watcher
	opt    Options
	coffre string
	git    []string
	actifs map[string]bool
}

func (s *surveillance) log(format string, args ...any) {
	if s.opt.Journal != nil {
		s.opt.Journal(format, args...)
	}
}

func absolu(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return p
}

func relatif(root, path string) (string, bool) {
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

func (s *surveillance) arbre(root string, cible map[string]bool, cache bool) {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.log("Parcours de %s : %v", path, err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if cache && path != root && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		cible[path] = true
		// Poser la surveillance avant de parcourir les enfants ferme la fenêtre de création.
		s.ajouter(path)
		return nil
	})
	if err != nil {
		s.log("Surveillance de %s : %v", root, err)
	}
}

func (s *surveillance) ajouter(path string) {
	if s.actifs[path] {
		return
	}
	if err := s.w.Add(path); err != nil {
		s.log("Surveillance de %s : %v", path, err)
	} else {
		s.actifs[path] = true
	}
}

func (s *surveillance) depots() {
	s.git = nil
	if s.opt.Depots == nil {
		return
	}
	for _, depot := range s.opt.Depots() {
		git := filepath.Join(absolu(depot), ".git")
		info, err := os.Stat(git)
		if err != nil {
			s.log("Dépôt %s : %v", depot, err)
			continue
		}
		if !info.IsDir() {
			data, err := os.ReadFile(git)
			if err != nil {
				s.log("Lecture de %s : %v", git, err)
				continue
			}
			line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
			if !strings.HasPrefix(line, "gitdir:") {
				s.log("Fichier gitdir invalide : %s", git)
				continue
			}
			dir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
			if dir == "" {
				s.log("Fichier gitdir vide : %s", git)
				continue
			}
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(filepath.Dir(git), dir)
			}
			git = absolu(dir)
		}
		s.git = append(s.git, git)
		// Les worktrees partagent les branches dans le répertoire commun.
		if data, err := os.ReadFile(filepath.Join(git, "commondir")); err == nil {
			common := strings.TrimSpace(string(data))
			if common != "" {
				if !filepath.IsAbs(common) {
					common = filepath.Join(git, common)
				}
				s.git = append(s.git, absolu(common))
			}
		}
	}
}

func (s *surveillance) ajuster() {
	cible := make(map[string]bool)
	if s.coffre != "" {
		s.arbre(s.coffre, cible, true)
	}
	for _, git := range s.git {
		cible[git] = true
		s.ajouter(git)
		refs := filepath.Join(git, "refs")
		if info, err := os.Stat(refs); err == nil && info.IsDir() {
			cible[refs] = true
			s.ajouter(refs)
		}
		heads := filepath.Join(refs, "heads")
		if _, err := os.Stat(heads); err == nil {
			s.arbre(heads, cible, false)
		}
	}
	for path := range s.actifs {
		if !cible[path] {
			_ = s.w.Remove(path)
			delete(s.actifs, path)
		}
	}
	for path := range cible {
		s.ajouter(path)
	}
}

func (s *surveillance) pertinent(event fsnotify.Event) bool {
	path := filepath.Clean(event.Name)
	info, statErr := os.Stat(path)
	dossier := (statErr == nil && info.IsDir()) || s.actifs[path]
	if dossier && !event.Has(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) {
		// Windows signale aussi l'écriture du dossier parent : seul son contenu compte.
		return false
	}
	if s.coffre != "" {
		rel, ok := relatif(s.coffre, path)
		parents := filepath.Dir(rel)
		if dossier {
			parents = rel
		}
		if ok && visible(parents) {
			if statErr == nil && info.IsDir() {
				// Un fichier peut précéder l'installation de la surveillance du dossier.
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
			return strings.EqualFold(filepath.Ext(path), ".md") || (s.actifs[path] && event.Has(fsnotify.Remove|fsnotify.Rename))
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
		if rel, ok := relatif(filepath.Join(git, "refs", "heads"), path); ok {
			return !strings.HasSuffix(rel, ".lock")
		}
	}
	return false
}

// Surveiller bloque jusqu'à l'annulation du contexte, sans import concurrent.
func Surveiller(ctx context.Context, opt Options) error {
	if ctx.Err() != nil {
		return nil
	}
	if opt.Importer == nil {
		return fmt.Errorf("watch : Importer est requis")
	}
	if opt.Delai < 0 {
		return fmt.Errorf("watch : Delai doit être positif")
	}
	if opt.Delai == 0 {
		opt.Delai = 1500 * time.Millisecond
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	s := &surveillance{w: w, opt: opt, actifs: make(map[string]bool)}
	if opt.Coffre != "" {
		s.coffre = absolu(opt.Coffre)
	}
	s.depots()
	s.ajuster()
	type resultat struct {
		c   Changements
		err error
	}
	fin := make(chan resultat, 1)
	var timer *time.Timer
	var attente <-chan time.Time
	enCours, suivant := false, false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	lancer := func() {
		enCours = true
		go func() { c, err := opt.Importer(ctx); fin <- resultat{c, err} }()
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
			// Les suppressions se reconnaissent encore dans l'ancienne liste.
			supprime := s.actifs[event.Name] && event.Has(fsnotify.Remove|fsnotify.Rename)
			pertinent := false
			if supprime {
				pertinent = s.pertinent(event)
				// fsnotify retire automatiquement les dossiers supprimés ou renommés.
				for path := range s.actifs {
					if _, ok := relatif(event.Name, path); ok {
						_ = s.w.Remove(path)
						delete(s.actifs, path)
					}
				}
			}
			info, statErr := os.Stat(event.Name)
			if supprime || (statErr == nil && info.IsDir() && event.Has(fsnotify.Create|fsnotify.Rename)) {
				s.ajuster()
			}
			pertinent = s.pertinent(event) || pertinent
			if !pertinent {
				continue
			}
			if enCours {
				suivant = true
			} else if attente == nil {
				timer = time.NewTimer(opt.Delai)
				attente = timer.C
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			s.log("Erreur fsnotify : %v", err)
		case <-attente:
			attente = nil
			if ctx.Err() == nil {
				lancer()
			}
		case result := <-fin:
			enCours = false
			if result.err != nil {
				s.log("Erreur d'import : %v", result.err)
			} else if !result.c.Vide() && opt.Annoncer != nil {
				opt.Annoncer(result.c.Tables()...)
			}
			s.depots()
			s.ajuster()
			if suivant && ctx.Err() == nil {
				suivant = false
				lancer()
			}
		}
	}
}

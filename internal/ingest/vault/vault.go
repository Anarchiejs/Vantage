// Package vault reads Markdown notes without writing to the vault or database.
package vault

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/model"
	"gopkg.in/yaml.v3"
)

type Resultat struct {
	Notes    []model.Note
	Fiches   []model.Fiche
	Sessions []model.Session
	Journal  []model.EntreeJournal
}

// Lire walks Markdown files in lexical order, skipping hidden directories.
func Lire(racine string) (Resultat, error) {
	var result Resultat
	err := filepath.WalkDir(racine, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if file != racine && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || filepath.Ext(file) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(racine, file)
		if err != nil {
			return err
		}
		note, meta, err := lire(racine, rel)
		if err != nil {
			return err
		}
		result.Notes = append(result.Notes, note)
		parts := strings.Split(note.Chemin, "/")
		if len(parts) == 3 && parts[0] == "Projets" && parts[2] == parts[1]+".md" && note.Type == "projet" {
			source := meta.first("chemin_source")
			if !absolutePath(source) {
				source = ""
			}
			fiche := model.Fiche{Nom: note.Titre, Statut: meta.first("statut"), CheminSource: source,
				Maj: note.Maj, VerifieLe: meta.first("verifie_le"), VerifieCommit: meta.first("verifie_commit")}
			for _, techno := range meta.values("technos") {
				if target := cible(techno); target != "" {
					fiche.Technos = append(fiche.Technos, target)
				}
			}
			result.Fiches = append(result.Fiches, fiche)
		}
		if note.Dossier == "Sessions" && note.Type == "session" {
			result.Sessions = append(result.Sessions, model.Session{CheminNote: note.Chemin,
				Date: meta.first("date"), Projet: cible(meta.first("projet")), IA: cible(meta.first("ia"))})
		}
		if note.Chemin == "Journal.md" {
			result.Journal = journal(note.Contenu)
		}
		return nil
	})
	return result, err
}

// LireNote accepts either a vault-relative path or an absolute path within it.
func LireNote(racine, chemin string) (model.Note, error) {
	note, _, err := lire(racine, chemin)
	return note, err
}

func lire(racine, chemin string) (model.Note, metadata, error) {
	var note model.Note
	root, err := filepath.Abs(racine)
	if err != nil {
		return note, nil, err
	}
	file := filepath.FromSlash(chemin)
	if !filepath.IsAbs(file) {
		file = filepath.Join(root, file)
	}
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return note, nil, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.Ext(rel) != ".md" {
		return note, nil, fmt.Errorf("note outside vault or not Markdown: %s", chemin)
	}
	// Do not follow symlinks, including intermediate directory symlinks.
	current := root
	segments := strings.Split(rel, string(filepath.Separator))
	for i, segment := range segments {
		if i < len(segments)-1 && strings.HasPrefix(segment, ".") {
			return note, nil, fmt.Errorf("hidden directory: %s", chemin)
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return note, nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return note, nil, fmt.Errorf("symlink note: %s", chemin)
		}
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return note, nil, err
	}
	body, meta := frontMatter(strings.TrimPrefix(string(raw), "\ufeff"))
	note = model.Note{Chemin: filepath.ToSlash(rel), Titre: strings.TrimSuffix(filepath.Base(rel), ".md"),
		Type: meta.first("type"), Maj: meta.first("maj"), Hash: fmt.Sprintf("%x", sha256.Sum256(raw)), Contenu: body, Liens: liens(body)}
	if len(segments) > 1 {
		note.Dossier = segments[0]
	}
	return note, meta, nil
}

type metadata map[string]yaml.Node

func (m metadata) values(key string) []string {
	node, ok := m[key]
	if !ok {
		return nil
	}
	if node.Kind == yaml.ScalarNode {
		return []string{node.Value}
	}
	var result []string
	if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				result = append(result, item.Value)
			}
		}
	}
	return result
}

func (m metadata) first(key string) string {
	values := m.values(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func frontMatter(text string) (string, metadata) {
	lines := strings.SplitAfter(text, "\n")
	if strings.TrimRight(lines[0], "\r\n") != "---" {
		return text, nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") != "---" {
			continue
		}
		body := strings.Join(lines[i+1:], "")
		var meta metadata
		if yaml.Unmarshal([]byte(strings.Join(lines[1:i], "")), &meta) != nil {
			meta = nil
		}
		return body, meta
	}
	// An unclosed header is malformed; retain the text for search.
	return text, nil
}

func absolutePath(value string) bool {
	return strings.HasPrefix(value, "/") || (len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '/' || value[2] == '\\'))
}

func cible(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "!")
	value = strings.TrimPrefix(value, "[[")
	value = strings.TrimSuffix(value, "]]")
	value = strings.SplitN(strings.SplitN(value, "|", 2)[0], "#", 2)[0]
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = path.Base(strings.ReplaceAll(value, "\\", "/"))
	ext := path.Ext(value)
	if ext != "" && ext != ".md" {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(value, ".md"))
}

// liens ignores fenced code and backtick spans (including multiline spans).
func liens(body string) []string {
	var result []string
	seen := map[string]bool{}
	var fence byte
	fenceLen, inline := 0, 0
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if indent <= 3 && len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			n := 0
			for n < len(trimmed) && trimmed[n] == trimmed[0] {
				n++
			}
			if fence != 0 {
				if trimmed[0] == fence && n >= fenceLen && strings.TrimSpace(trimmed[n:]) == "" {
					fence = 0
				}
				continue
			}
			if n >= 3 && inline == 0 {
				fence, fenceLen = trimmed[0], n
				continue
			}
		}
		if fence != 0 {
			continue
		}
		for i := 0; i < len(line); {
			if line[i] == '\\' && inline == 0 && i+1 < len(line) {
				i += 2
				continue
			}
			if line[i] == '`' {
				n := 1
				for i+n < len(line) && line[i+n] == '`' {
					n++
				}
				if inline == 0 {
					inline = n
				} else if inline == n {
					inline = 0
				}
				i += n
				continue
			}
			if inline == 0 && strings.HasPrefix(line[i:], "[[") {
				end := strings.Index(line[i+2:], "]]")
				if end >= 0 {
					target := cible(line[i+2 : i+2+end])
					if target != "" && !seen[target] {
						seen[target] = true
						result = append(result, target)
					}
					i += end + 4
					continue
				}
			}
			i++
		}
	}
	return result
}

var journalLine = regexp.MustCompile(`^- (\d{4}-\d{2}-\d{2}) - \[\[([^\]]+)\]\] - (.*)$`)

func journal(body string) []model.EntreeJournal {
	var result []model.EntreeJournal
	for _, line := range strings.Split(body, "\n") {
		match := journalLine.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if match == nil {
			continue
		}
		if _, err := time.Parse("2006-01-02", match[1]); err != nil {
			continue
		}
		result = append(result, model.EntreeJournal{Date: match[1], IA: cible(match[2]), Texte: match[3]})
	}
	return result
}

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
	"sort"
	"strings"
	"time"

	"github.com/Anarchiejs/Vantage/internal/model"
	"github.com/Anarchiejs/Vantage/internal/ospath"
	"gopkg.in/yaml.v3"
)

type Result struct {
	Notes    []model.Note
	Cards    []model.ProjectCard
	Sessions []model.Session
	Journal  []model.JournalEntry
}

// Read walks Markdown files in lexical order, skipping hidden directories.
func Read(root string) (Result, error) {
	var result Result
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if file != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(file), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		note, meta, err := read(root, rel)
		if err != nil {
			return err
		}
		result.Notes = append(result.Notes, note)
		parts := strings.Split(note.Path, "/")
		if len(parts) == 3 && parts[0] == "Projects" && parts[2] == parts[1]+".md" && note.Type == "project" {
			source := meta.first("source_path")
			if !absolutePath(source) {
				source = ""
			}
			card := model.ProjectCard{Name: note.Title, Status: meta.first("status"), SourcePath: source,
				Branch: strings.TrimSpace(meta.first("branch")), Updated: note.Updated, VerifiedOn: meta.first("verified_on"),
				VerifiedCommit: leadingSHA(meta.first("verified_commit")), ExpectedNotes: meta.values("expected_notes"),
				Extra: meta.repos()}
			for _, technology := range meta.values("technologies") {
				if target := linkTarget(technology); target != "" {
					card.Technologies = append(card.Technologies, target)
				}
			}
			result.Cards = append(result.Cards, card)
		}
		if note.Folder == "Sessions" && note.Type == "session" {
			result.Sessions = append(result.Sessions, model.Session{NotePath: note.Path,
				Date: meta.first("date"), Project: linkTarget(meta.first("project")), AI: linkTarget(meta.first("ai"))})
		}
		if note.Path == "Journal.md" {
			result.Journal = journal(note.Content)
		}
		return nil
	})
	return result, err
}

// ReadNote accepts either a vault-relative path or an absolute path within it.
func ReadNote(root, notePath string) (model.Note, error) {
	note, _, err := read(root, notePath)
	return note, err
}

func read(root, notePath string) (model.Note, metadata, error) {
	var note model.Note
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return note, nil, err
	}
	file := filepath.FromSlash(notePath)
	if !filepath.IsAbs(file) {
		file = filepath.Join(absRoot, file)
	}
	rel, err := filepath.Rel(absRoot, file)
	if err != nil {
		return note, nil, err
	}
	// macOS stores filenames decomposed. NFC matches Linux, Windows, and the
	// bytes in this repository. On those systems ForCompare returns rel unchanged.
	rel = ospath.ForCompare(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || !strings.EqualFold(filepath.Ext(rel), ".md") {
		return note, nil, fmt.Errorf("note outside vault or not Markdown: %s", notePath)
	}
	// Do not follow symlinks, including intermediate directory symlinks.
	current := absRoot
	segments := strings.Split(rel, string(filepath.Separator))
	for i, segment := range segments {
		if i < len(segments)-1 && strings.HasPrefix(segment, ".") {
			return note, nil, fmt.Errorf("hidden directory: %s", notePath)
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return note, nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return note, nil, fmt.Errorf("symlink note: %s", notePath)
		}
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return note, nil, err
	}
	body, meta := frontMatter(strings.TrimPrefix(string(raw), "\ufeff"))
	title := filepath.Base(rel)
	if ext := filepath.Ext(title); strings.EqualFold(ext, ".md") {
		title = title[:len(title)-len(ext)]
	}
	note = model.Note{Path: filepath.ToSlash(rel), Title: title,
		Type: meta.first("type"), Updated: meta.first("updated"), Hash: fmt.Sprintf("%x", sha256.Sum256(raw)), Content: body,
		Links: links(body), PropertyLinks: propertyLinks(meta)}
	if len(segments) > 1 {
		note.Folder = segments[0]
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

// repos reads a sequence of paths, or of maps with path and branch.
func (m metadata) repos() []model.RepoRef {
	node, ok := m["repos"]
	if !ok || node.Kind != yaml.SequenceNode {
		return nil
	}
	var result []model.RepoRef
	for _, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			if path := strings.TrimSpace(item.Value); path != "" && absolutePath(path) {
				result = append(result, model.RepoRef{Path: path})
			}
		case yaml.MappingNode:
			ref := model.RepoRef{}
			for i := 0; i+1 < len(item.Content); i += 2 {
				key := item.Content[i].Value
				val := strings.TrimSpace(item.Content[i+1].Value)
				switch key {
				case "path":
					ref.Path = val
				case "branch":
					ref.Branch = val
				}
			}
			if absolutePath(ref.Path) {
				result = append(result, ref)
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

// cleanWikilink keeps a path (`Folder/Note`) and drops the alias, heading, and block id.
// linkTarget is the note title: the last path segment of that target.
func cleanWikilink(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "!")
	value = strings.TrimPrefix(value, "[[")
	value = strings.TrimSuffix(value, "]]")
	value = strings.SplitN(value, "|", 2)[0]
	value = strings.SplitN(value, "#", 2)[0]
	value = strings.SplitN(value, "^", 2)[0]
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return ""
	}
	ext := path.Ext(value)
	if ext != "" && !strings.EqualFold(ext, ".md") {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(value, ext))
}

func linkTarget(value string) string {
	cleaned := cleanWikilink(value)
	if cleaned == "" {
		return ""
	}
	return path.Base(cleaned)
}

func propertyLinks(meta metadata) []string {
	if meta == nil {
		return nil
	}
	var result []string
	seen := map[string]bool{}
	var walk func(yaml.Node)
	walk = func(node yaml.Node) {
		if node.Kind == yaml.ScalarNode {
			if !strings.Contains(node.Value, "[[") {
				return
			}
			target := cleanWikilink(node.Value)
			if target == "" || seen[target] {
				return
			}
			seen[target] = true
			result = append(result, target)
			return
		}
		for _, child := range node.Content {
			walk(*child)
		}
	}
	keys := make([]string, 0, len(meta))
	for key := range meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		walk(meta[key])
	}
	sort.Strings(result)
	return result
}

// links ignores fenced code and backtick spans (including multiline spans).
func links(body string) []string {
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
					target := cleanWikilink(line[i+2 : i+2+end])
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

// leadingSHA keeps the sha at the start of verified_commit ("0df64eb2 (official upstream)" -> "0df64eb2");
// "" when the value does not start with a sha ("none", "not verified").
func leadingSHA(value string) string {
	return strings.ToLower(strings.TrimSpace(shaStart.FindString(strings.TrimSpace(value))))
}

var shaStart = regexp.MustCompile(`(?i)^[0-9a-f]{7,40}(?:\s|$)`)

var journalLine = regexp.MustCompile(`^[-*] (\d{4}-\d{2}-\d{2}) - \[\[([^\]]+)\]\] - (.*)$`)

func journal(body string) []model.JournalEntry {
	var result []model.JournalEntry
	for _, line := range strings.Split(body, "\n") {
		match := journalLine.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if match == nil {
			continue
		}
		if _, err := time.Parse("2006-01-02", match[1]); err != nil {
			continue
		}
		result = append(result, model.JournalEntry{Date: match[1], AI: linkTarget(match[2]), Text: match[3]})
	}
	return result
}

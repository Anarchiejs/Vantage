// Package ingest writes to the database what the git, vault, and code readers produce.
// One transaction for the vault, then one per repository; everything is idempotent.
package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	icode "github.com/Anarchiejs/Vantage/internal/ingest/code"
	igit "github.com/Anarchiejs/Vantage/internal/ingest/git"
	ivault "github.com/Anarchiejs/Vantage/internal/ingest/vault"
	"github.com/Anarchiejs/Vantage/internal/model"
	"github.com/Anarchiejs/Vantage/internal/sql/gen"
)

// Options for an import.
type Options struct {
	Vault string
	// Repos replaces a card's source_path (project name to directory); used by tests.
	Repos map[string]string
	// Today dates language measurements; empty means today's date.
	Today string
}

// Summary of an import.
type Summary struct {
	NotesWritten   int
	NotesUnchanged int
	NotesDeleted   int
	Repos          int
	Commits        int
	Duration       time.Duration
	Warnings       []string
}

// Import reads the vault, then each repository named by a card, and updates the database.
func Import(ctx context.Context, base *sql.DB, opt Options) (Summary, error) {
	started := time.Now()
	var summary Summary
	if opt.Today == "" {
		opt.Today = time.Now().Format("2006-01-02")
	}

	res, err := ivault.Read(opt.Vault)
	if err != nil {
		return summary, fmt.Errorf("read vault: %w", err)
	}
	projects, err := writeVault(ctx, base, res, opt.Vault, &summary)
	if err != nil {
		return summary, fmt.Errorf("write vault: %w", err)
	}

	// Read repositories in parallel (git and scc dominate the time), write one by one:
	// SQLite accepts only one writer at a time.
	var reads []*repoRead
	for _, card := range res.Cards {
		specs := append([]model.RepoRef{}, card.Extra...)
		primary := card.SourcePath
		if replacement, ok := opt.Repos[card.Name]; ok {
			primary = replacement
		}
		if primary != "" {
			specs = append([]model.RepoRef{{Path: primary, Branch: card.Branch}}, specs...)
		}
		seenPath := map[string]bool{}
		for _, spec := range specs {
			if spec.Path == "" || seenPath[spec.Path] {
				continue
			}
			seenPath[spec.Path] = true
			branch := spec.Branch
			if branch == "" {
				branch = card.Branch
			}
			reads = append(reads, &repoRead{project: card.Name, path: spec.Path, requested: branch})
		}
	}
	q := gen.New(base)
	var wg sync.WaitGroup
	slots := make(chan struct{}, runtime.NumCPU())
	for _, read := range reads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			read.load(ctx, q)
		}()
	}
	wg.Wait()

	for _, read := range reads {
		added, err := 0, read.err
		if err == nil {
			added, err = writeRepository(ctx, base, projects[read.project], read, opt.Today)
		}
		summary.Warnings = append(summary.Warnings, read.warnings...)
		if err != nil {
			// A missing or broken repository must not block the others.
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s (%s): %v", read.project, read.path, err))
			continue
		}
		summary.Repos++
		summary.Commits += added
	}
	if err := dropUnusedRepositories(ctx, base, projects, reads); err != nil {
		return summary, err
	}
	summary.Duration = time.Since(started)
	return summary, nil
}

func writeVault(ctx context.Context, base *sql.DB, res ivault.Result, vault string, summary *Summary) (map[string]int64, error) {
	tx, err := base.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Prepared once per transaction: SQLite does not recompile the SQL on every row.
	q, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	defer q.Close()

	projects := map[string]int64{}
	for _, card := range res.Cards {
		id, err := q.UpsertProject(ctx, gen.UpsertProjectParams{
			Name:           card.Name,
			Status:         empty(card.Status),
			SourcePath:     empty(card.SourcePath),
			CardUpdated:    empty(card.Updated),
			VerifiedCommit: empty(card.VerifiedCommit),
			VerifiedOn:     empty(card.VerifiedOn),
			Branch:         empty(card.Branch),
			ExpectedNotes:  encodeList(card.ExpectedNotes),
		})
		if err != nil {
			return nil, fmt.Errorf("project %s: %w", card.Name, err)
		}
		projects[card.Name] = id
	}
	kept := map[string]bool{}
	for name := range projects {
		kept[name] = true
	}
	previousProjects, err := q.ListProjectIDs(ctx)
	if err != nil {
		return nil, err
	}
	for _, project := range previousProjects {
		if kept[project.Name] {
			continue
		}
		repos, err := q.ProjectRepositories(ctx, project.ID)
		if err != nil {
			return nil, err
		}
		for _, repo := range repos {
			if err := q.DeleteCommitSearchForRepository(ctx, repo.ID); err != nil {
				return nil, err
			}
		}
		if err := q.DeleteProject(ctx, project.ID); err != nil {
			return nil, err
		}
	}

	existing, err := q.ListNotes(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]int64, len(existing))
	for _, note := range existing {
		ids[note.Path] = note.ID
	}
	seen := make(map[string]bool, len(res.Notes))
	for _, note := range res.Notes {
		seen[note.Path] = true
		id, err := writeNote(ctx, q, note)
		if err != nil {
			return nil, fmt.Errorf("note %s: %w", note.Path, err)
		}
		if id == 0 {
			summary.NotesUnchanged++
			continue
		}
		ids[note.Path] = id
		summary.NotesWritten++
	}
	for _, note := range existing {
		if seen[note.Path] {
			continue
		}
		if err := q.DeleteSearch(ctx, gen.DeleteSearchParams{Source: "note", Ref: note.Path}); err != nil {
			return nil, err
		}
		if err := q.DeleteNote(ctx, note.ID); err != nil {
			return nil, err
		}
		summary.NotesDeleted++
	}
	catalog := noteCatalog(res.Notes, ids)
	if err := resolveStoredLinks(ctx, q, catalog); err != nil {
		return nil, err
	}
	if err := q.DeleteSessions(ctx); err != nil {
		return nil, err
	}
	if err := q.DeleteJournal(ctx); err != nil {
		return nil, err
	}
	names := map[string]int64{}
	for name, id := range projects {
		names[strings.ToLower(name)] = id
	}
	for _, session := range res.Sessions {
		id, ok := ids[session.NotePath]
		if !ok {
			continue
		}
		var projectID *int64
		projectName := canonicalTitle(session.Project, catalog)
		if id, ok := names[strings.ToLower(projectName)]; ok {
			projectID = &id
		} else if id, ok := names[strings.ToLower(session.Project)]; ok {
			projectID = &id
		}
		ai := canonicalTitle(session.AI, catalog)
		if err := q.UpsertSession(ctx, gen.UpsertSessionParams{NoteID: id, Date: session.Date, ProjectID: projectID, Ai: empty(ai)}); err != nil {
			return nil, err
		}
	}
	for _, entry := range res.Journal {
		if err := q.InsertJournal(ctx, gen.InsertJournalParams{Date: entry.Date, Ai: empty(canonicalTitle(entry.AI, catalog)), Body: entry.Text}); err != nil {
			return nil, err
		}
	}
	for _, card := range res.Cards {
		id := projects[card.Name]
		if err := q.ClearProjectTechnologies(ctx, id); err != nil {
			return nil, err
		}
		seenTech := map[string]bool{}
		for _, technology := range card.Technologies {
			name := canonicalTitle(technology, catalog)
			if name == "" || seenTech[strings.ToLower(name)] {
				continue
			}
			seenTech[strings.ToLower(name)] = true
			technologyID, err := q.UpsertTechnology(ctx, gen.UpsertTechnologyParams{Name: name})
			if err != nil {
				return nil, err
			}
			if err := q.LinkProjectTechnology(ctx, gen.LinkProjectTechnologyParams{ProjectID: id, TechnologyID: technologyID}); err != nil {
				return nil, err
			}
		}
	}
	if err := q.DeleteUnusedTechnologies(ctx); err != nil {
		return nil, err
	}
	if vault != "" {
		abs, err := filepath.Abs(vault)
		if err != nil {
			return nil, err
		}
		if err := q.UpsertSetting(ctx, gen.UpsertSettingParams{Key: "vault_path", Value: abs}); err != nil {
			return nil, err
		}
		if err := q.UpsertSetting(ctx, gen.UpsertSettingParams{Key: "vault_name", Value: filepath.Base(abs)}); err != nil {
			return nil, err
		}
	}
	return projects, tx.Commit()
}

// writeNote returns 0 when the note is unchanged since the last import.
func writeNote(ctx context.Context, q *gen.Queries, note model.Note) (int64, error) {
	if previous, err := q.NoteHash(ctx, note.Path); err == nil && previous.ContentHash == note.Hash {
		if err := replaceLinks(ctx, q, previous.ID, note); err != nil {
			return 0, err
		}
		return 0, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	id, err := q.UpsertNote(ctx, gen.UpsertNoteParams{
		Path: note.Path, Folder: note.Folder, Title: note.Title,
		Type: empty(note.Type), Updated: empty(note.Updated), ContentHash: note.Hash,
	})
	if err != nil {
		return 0, err
	}
	if err := replaceLinks(ctx, q, id, note); err != nil {
		return 0, err
	}
	if err := q.DeleteSearch(ctx, gen.DeleteSearchParams{Source: "note", Ref: note.Path}); err != nil {
		return 0, err
	}
	err = q.InsertSearch(ctx, gen.InsertSearchParams{Title: note.Title, Content: note.Content, Source: "note", Ref: note.Path})
	return id, err
}

// repoRead is what we know about a repository before writing it.
type repoRead struct {
	project, path, requested string
	head, branch             string
	unchanged                bool // tip is already imported: nothing to write
	rewritten                bool // history changed under last_imported_sha
	commits                  []model.Commit
	stats                    []model.LanguageStat
	warnings                 []string
	err                      error
}

func (r *repoRead) load(ctx context.Context, q *gen.Queries) {
	r.head, r.branch, r.err = igit.Tip(ctx, r.path, r.requested)
	if r.err != nil {
		return
	}
	since := ""
	if sha, err := q.LastRepositorySHA(ctx, r.path); err == nil && sha != nil {
		since = *sha
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		r.err = err
		return
	}
	if since == r.head {
		r.unchanged = true
		return
	}
	r.commits, r.err = igit.ReadRevision(ctx, r.path, r.head, since)
	if errors.Is(r.err, igit.ErrHistoryRewritten) {
		r.rewritten, r.err = true, nil
	}
	if r.err != nil {
		return
	}
	stats, warnings, err := icode.Measure(ctx, r.path, r.head)
	r.warnings = warnings
	if err != nil {
		r.warnings = append(r.warnings, fmt.Sprintf("language measurement: %v", err))
		return
	}
	r.stats = stats
}

func writeRepository(ctx context.Context, base *sql.DB, projectID int64, read *repoRead, today string) (int, error) {
	tx, err := base.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	q, err := gen.Prepare(ctx, tx)
	if err != nil {
		return 0, err
	}
	defer q.Close()

	repository, err := q.UpsertRepository(ctx, gen.UpsertRepositoryParams{ProjectID: projectID, Path: read.path, Branch: empty(read.branch)})
	if err != nil {
		return 0, err
	}
	if read.unchanged {
		return 0, tx.Commit()
	}
	if read.rewritten {
		if err := q.DeleteRepositoryCommitSearch(ctx, repository.ID); err != nil {
			return 0, err
		}
		if err := q.DeleteRepositoryCommits(ctx, repository.ID); err != nil {
			return 0, err
		}
	}

	added := 0
	for _, commit := range read.commits {
		inserted, err := q.InsertCommit(ctx, gen.InsertCommitParams{
			Sha: commit.SHA, RepositoryID: repository.ID, Author: commit.Author,
			DateUtc: commit.DateUTC(), OffsetMin: commit.OffsetMin(), Message: commit.Message,
		})
		if err != nil {
			return 0, fmt.Errorf("commit %s: %w", commit.SHA, err)
		}
		if inserted == 0 {
			continue // this repository already stored the commit
		}
		added++
		for _, file := range commit.Files {
			if err := q.InsertChangedFile(ctx, gen.InsertChangedFileParams{
				RepositoryID: repository.ID, CommitSha: commit.SHA, Path: file.Path, Language: empty(file.Language),
				Additions: file.Additions, Deletions: file.Deletions,
			}); err != nil {
				return 0, err
			}
		}
		title, _, _ := strings.Cut(commit.Message, "\n")
		ref := fmt.Sprintf("%d:%s", repository.ID, commit.SHA)
		if err := q.InsertSearch(ctx, gen.InsertSearchParams{Title: title, Content: commit.Message, Source: "commit", Ref: ref}); err != nil {
			return 0, err
		}
	}

	for _, stat := range read.stats {
		if err := q.InsertLanguageStat(ctx, gen.InsertLanguageStatParams{
			RepositoryID: repository.ID, MeasuredOn: today, Language: stat.Language,
			Files: stat.Files, CodeLines: stat.CodeLines, CommentLines: stat.CommentLines,
		}); err != nil {
			return 0, err
		}
	}

	if err := q.MarkRepositoryImported(ctx, gen.MarkRepositoryImportedParams{LastImportedSha: &read.head, ID: repository.ID}); err != nil {
		return 0, err
	}
	return added, tx.Commit()
}

func empty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func encodeList(items []string) *string {
	if len(items) == 0 {
		return nil
	}
	body, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	text := string(body)
	return &text
}

func replaceLinks(ctx context.Context, q *gen.Queries, id int64, note model.Note) error {
	if err := q.DeleteNoteLinks(ctx, id); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, link := range note.Links {
		if link == "" || seen[link] {
			continue
		}
		seen[link] = true
		if err := q.InsertLink(ctx, gen.InsertLinkParams{SourceID: id, TargetTitle: link, FromProperty: 0}); err != nil {
			return err
		}
	}
	for _, link := range note.PropertyLinks {
		if link == "" || seen[link] {
			continue
		}
		seen[link] = true
		if err := q.InsertLink(ctx, gen.InsertLinkParams{SourceID: id, TargetTitle: link, FromProperty: 1}); err != nil {
			return err
		}
	}
	return nil
}

type noteRef struct {
	id     int64
	path   string
	folder string
	title  string
}

func noteCatalog(notes []model.Note, ids map[string]int64) []noteRef {
	var catalog []noteRef
	for _, note := range notes {
		id, ok := ids[note.Path]
		if !ok {
			continue
		}
		folder := note.Folder
		if folder == "" {
			if i := strings.IndexByte(note.Path, '/'); i >= 0 {
				folder = note.Path[:i]
			}
		}
		catalog = append(catalog, noteRef{id: id, path: note.Path, folder: folder, title: note.Title})
	}
	return catalog
}

func resolveTarget(target, sourcePath string, catalog []noteRef) (int64, bool) {
	want := strings.TrimSuffix(target, ".md")
	var pathHits []noteRef
	for _, note := range catalog {
		if strings.EqualFold(strings.TrimSuffix(note.path, ".md"), want) {
			pathHits = append(pathHits, note)
		}
	}
	if len(pathHits) == 1 {
		return pathHits[0].id, true
	}
	title := path.Base(want)
	var titleHits []noteRef
	for _, note := range catalog {
		if strings.EqualFold(note.title, title) {
			titleHits = append(titleHits, note)
		}
	}
	if len(titleHits) == 0 {
		return 0, false
	}
	if len(titleHits) == 1 {
		return titleHits[0].id, true
	}
	sourceFolder := ""
	if i := strings.IndexByte(sourcePath, '/'); i >= 0 {
		sourceFolder = sourcePath[:i]
	}
	var same []noteRef
	for _, note := range titleHits {
		if note.folder == sourceFolder {
			same = append(same, note)
		}
	}
	pool := titleHits
	if len(same) == 1 {
		return same[0].id, true
	}
	if len(same) > 1 {
		pool = same
	}
	sort.Slice(pool, func(i, j int) bool {
		if len(pool[i].path) != len(pool[j].path) {
			return len(pool[i].path) < len(pool[j].path)
		}
		return pool[i].path < pool[j].path
	})
	return pool[0].id, true
}

func canonicalTitle(name string, catalog []noteRef) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	id, ok := resolveTarget(name, "", catalog)
	if !ok {
		return name
	}
	for _, note := range catalog {
		if note.id == id {
			return note.title
		}
	}
	return name
}

func resolveStoredLinks(ctx context.Context, q *gen.Queries, catalog []noteRef) error {
	rows, err := q.ListLinks(ctx)
	if err != nil {
		return err
	}
	byID := map[int64]noteRef{}
	for _, note := range catalog {
		byID[note.id] = note
	}
	for _, row := range rows {
		source := ""
		if note, ok := byID[row.SourceID]; ok {
			source = note.path
		}
		var target *int64
		if id, ok := resolveTarget(row.TargetTitle, source, catalog); ok {
			target = &id
		}
		if err := q.SetLinkTarget(ctx, gen.SetLinkTargetParams{TargetID: target, SourceID: row.SourceID, TargetTitle: row.TargetTitle}); err != nil {
			return err
		}
	}
	return nil
}

func dropUnusedRepositories(ctx context.Context, base *sql.DB, projects map[string]int64, reads []*repoRead) error {
	wanted := map[int64]map[string]bool{}
	for _, id := range projects {
		wanted[id] = map[string]bool{}
	}
	for _, read := range reads {
		id, ok := projects[read.project]
		if !ok {
			continue
		}
		wanted[id][read.path] = true
	}
	tx, err := base.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q, err := gen.Prepare(ctx, tx)
	if err != nil {
		return err
	}
	defer q.Close()
	for id, paths := range wanted {
		repos, err := q.ProjectRepositories(ctx, id)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if paths[repo.Path] {
				continue
			}
			if err := q.DeleteCommitSearchForRepository(ctx, repo.ID); err != nil {
				return err
			}
			if err := q.DeleteRepositoryCommits(ctx, repo.ID); err != nil {
				return err
			}
			if err := q.DeleteRepository(ctx, repo.ID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

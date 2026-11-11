// Package ingest writes to the database what the git, vault, and code readers produce.
// One transaction for the vault, then one per repository; everything is idempotent.
package ingest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	icode "github.com/Anarchiejs/Observatoire/internal/ingest/code"
	igit "github.com/Anarchiejs/Observatoire/internal/ingest/git"
	ivault "github.com/Anarchiejs/Observatoire/internal/ingest/vault"
	"github.com/Anarchiejs/Observatoire/internal/model"
	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
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
	projects, err := writeVault(ctx, base, res, &summary)
	if err != nil {
		return summary, fmt.Errorf("write vault: %w", err)
	}

	// Read repositories in parallel (git and scc dominate the time), write one by one:
	// SQLite accepts only one writer at a time.
	var reads []*repoRead
	for _, card := range res.Cards {
		path := card.SourcePath
		if replacement, ok := opt.Repos[card.Name]; ok {
			path = replacement
		}
		if path != "" {
			reads = append(reads, &repoRead{project: card.Name, path: path})
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
		if err != nil {
			// A missing or broken repository must not block the others.
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s (%s): %v", read.project, read.path, err))
			continue
		}
		summary.Repos++
		summary.Commits += added
	}
	summary.Duration = time.Since(started)
	return summary, nil
}

func writeVault(ctx context.Context, base *sql.DB, res ivault.Result, summary *Summary) (map[string]int64, error) {
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
		})
		if err != nil {
			return nil, fmt.Errorf("project %s: %w", card.Name, err)
		}
		projects[card.Name] = id
		for _, technology := range card.Technologies {
			technologyID, err := q.UpsertTechnology(ctx, gen.UpsertTechnologyParams{Name: technology})
			if err != nil {
				return nil, err
			}
			if err := q.LinkProjectTechnology(ctx, gen.LinkProjectTechnologyParams{ProjectID: id, TechnologyID: technologyID}); err != nil {
				return nil, err
			}
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
	if err := q.ResolveLinks(ctx); err != nil {
		return nil, err
	}

	for _, session := range res.Sessions {
		id, ok := ids[session.NotePath]
		if !ok {
			continue
		}
		var projectID *int64
		if id, ok := projects[session.Project]; ok {
			projectID = &id
		}
		if err := q.UpsertSession(ctx, gen.UpsertSessionParams{NoteID: id, Date: session.Date, ProjectID: projectID, Ai: empty(session.AI)}); err != nil {
			return nil, err
		}
	}
	for _, entry := range res.Journal {
		if err := q.InsertJournal(ctx, gen.InsertJournalParams{Date: entry.Date, Ai: empty(entry.AI), Body: entry.Text}); err != nil {
			return nil, err
		}
	}
	return projects, tx.Commit()
}

// writeNote returns 0 when the note is unchanged since the last import.
func writeNote(ctx context.Context, q *gen.Queries, note model.Note) (int64, error) {
	if previous, err := q.NoteHash(ctx, note.Path); err == nil && previous.ContentHash == note.Hash {
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
	if err := q.DeleteNoteLinks(ctx, id); err != nil {
		return 0, err
	}
	for _, link := range note.Links {
		if err := q.InsertLink(ctx, gen.InsertLinkParams{SourceID: id, TargetTitle: link}); err != nil {
			return 0, err
		}
	}
	if err := q.DeleteSearch(ctx, gen.DeleteSearchParams{Source: "note", Ref: note.Path}); err != nil {
		return 0, err
	}
	err = q.InsertSearch(ctx, gen.InsertSearchParams{Title: note.Title, Content: note.Content, Source: "note", Ref: note.Path})
	return id, err
}

// repoRead is what we know about a repository before writing it.
type repoRead struct {
	project, path string
	head, branch  string
	unchanged     bool // HEAD is already imported: nothing to write
	rewritten     bool // history changed under last_imported_sha
	commits       []model.Commit
	stats         []model.LanguageStat
	err           error
}

func (r *repoRead) load(ctx context.Context, q *gen.Queries) {
	r.head, r.branch, r.err = igit.Head(ctx, r.path)
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
	r.commits, r.err = igit.Read(ctx, r.path, since)
	if errors.Is(r.err, igit.ErrHistoryRewritten) {
		r.rewritten, r.err = true, nil
	}
	if r.err != nil {
		return
	}
	if r.stats, r.err = icode.Measure(ctx, r.path); r.err != nil {
		r.err = fmt.Errorf("language measurement: %w", r.err)
	}
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
			continue // already known (same commit in another repository)
		}
		added++
		for _, file := range commit.Files {
			if err := q.InsertChangedFile(ctx, gen.InsertChangedFileParams{
				CommitSha: commit.SHA, Path: file.Path, Language: empty(file.Language),
				Additions: file.Additions, Deletions: file.Deletions,
			}); err != nil {
				return 0, err
			}
		}
		title, _, _ := strings.Cut(commit.Message, "\n")
		if err := q.InsertSearch(ctx, gen.InsertSearchParams{Title: title, Content: commit.Message, Source: "commit", Ref: commit.SHA}); err != nil {
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

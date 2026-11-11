// Package model holds the values passed from the readers (git, vault, code) to the database writer.
// Readers produce these values without touching the database; internal/ingest writes them through internal/sql/gen.
package model

import "time"

// Commit is one entry from `git log --numstat`.
type Commit struct {
	SHA     string
	Author  string
	Date    time.Time // author timezone; DateUTC and OffsetMin are derived from it
	Message string    // subject line plus body, without trailing whitespace
	Files   []ChangedFile
	IsMerge bool
}

// DateUTC returns the timestamp stored in the database: "2026-10-26T14:03:00Z".
func (c Commit) DateUTC() string { return c.Date.UTC().Format("2006-01-02T15:04:05Z") }

// OffsetMin returns the author timezone offset in minutes.
func (c Commit) OffsetMin() int64 {
	_, seconds := c.Date.Zone()
	return int64(seconds / 60)
}

// ChangedFile: Additions and Deletions are nil for a binary file.
type ChangedFile struct {
	Path      string // final path, separator "/" (for a rename: the new path)
	Language  string // "" when unknown
	Additions *int64
	Deletions *int64
}

// ProjectCard is the note `Projects/<Name>/<Name>.md` for a project.
type ProjectCard struct {
	Name           string
	Status         string
	SourcePath     string
	Updated        string
	VerifiedOn     string
	VerifiedCommit string
	Technologies   []string
}

// Note is one note from the vault.
type Note struct {
	Path    string // relative to the vault, separator "/"
	Folder  string // first path segment, "" at the vault root
	Title   string // file name without ".md"
	Type    string
	Updated string
	Hash    string   // sha256 hex of the raw content
	Content string   // text without front matter, for full-text search
	Links   []string // [[...]] targets, without alias or "#section", deduplicated
}

// Session is a note `Sessions/YYYY-MM-DD - <Project> - topic.md`.
type Session struct {
	NotePath string
	Date     string
	Project  string // project name, "" when unknown
	AI       string
}

// JournalEntry is one dated line from Journal.md.
type JournalEntry struct {
	Date string
	AI   string
	Text string
}

// LanguageStat is the scc count for one language in a repository.
type LanguageStat struct {
	Language     string
	Files        int64
	CodeLines    int64
	CommentLines int64
}

// Package api serves the UI and the JSON API.
//
// contract.go is the exact mirror of web/src/api/types.ts: every change is made on both sides.
// Nullable fields are pointers; lists are never nil (return []T{}) so JSON contains [] rather than null.
package api

// APIError is the body of every error response.
type APIError struct {
	Error string `json:"error"`
}

// ---- GET /api/home

type ActivityDay struct {
	Day       string   `json:"day"`
	Commits   int64    `json:"commits"`
	Additions int64    `json:"additions"`
	Deletions int64    `json:"deletions"`
	Sessions  int64    `json:"sessions"`
	Projects  []string `json:"projects"`
}

type Stats struct {
	Commits        int64 `json:"commits"`
	Additions      int64 `json:"additions"`
	Deletions      int64 `json:"deletions"`
	Sessions       int64 `json:"sessions"`
	ActiveProjects int64 `json:"active_projects"`
}

type ActiveProject struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	LastCommit *string `json:"last_commit"`
	Commits30d int64   `json:"commits_30d"`
}

type DormantProject struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	LastCommit *string `json:"last_commit"`
	Days       int64   `json:"days"`
}

type StaleCard struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	VerifiedCommit *string `json:"verified_commit"`
	LastSHA        *string `json:"last_sha"`
	CommitsSince   int64   `json:"commits_since"`
}

type Home struct {
	Calendar      []ActivityDay    `json:"calendar"`
	Week          Stats            `json:"week"`
	PreviousWeek  Stats            `json:"previous_week"`
	Month         Stats            `json:"month"`
	PreviousMonth Stats            `json:"previous_month"`
	Year          Stats            `json:"year"`
	PreviousYear  Stats            `json:"previous_year"`
	Active        []ActiveProject  `json:"active"`
	Dormant       []DormantProject `json:"dormant"`
	StaleCards    []StaleCard      `json:"stale_cards"`
	QuietHidden   int64            `json:"quiet_hidden"`
	RecentNotes   []RecentNote     `json:"recent_notes"`
}

// RecentNote is a dated note for the home page: a session date or the note's updated date.
type RecentNote struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Folder string `json:"folder"`
	Day    string `json:"day"`
}

// ---- GET /api/projects and /api/projects/{id}

type ProjectSummary struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Status          *string `json:"status"`
	Commits         int64   `json:"commits"`
	LastCommit      *string `json:"last_commit"`
	PrimaryLanguage *string `json:"primary_language"`
}

type TimelineEvent struct {
	Date  string  `json:"date"`
	Type  string  `json:"type"` // commit | session | journal
	Title string  `json:"title"`
	Ref   string  `json:"ref"`
	AI    *string `json:"ai"`
}

type WeeklyVolume struct {
	Week      string `json:"week"`
	Commits   int64  `json:"commits"`
	Additions int64  `json:"additions"`
	Deletions int64  `json:"deletions"`
}

type LanguageShare struct {
	Language  string  `json:"language"`
	CodeLines int64   `json:"code_lines"`
	Share     float64 `json:"share"`
}

type LanguagePoint struct {
	MeasuredOn string `json:"measured_on"`
	Language   string `json:"language"`
	CodeLines  int64  `json:"code_lines"`
}

type Hotspot struct {
	Path          string `json:"path"`
	Modifications int64  `json:"modifications"`
	Additions     int64  `json:"additions"`
	Deletions     int64  `json:"deletions"`
}

type Health struct {
	VerifiedCommit *string  `json:"verified_commit"`
	VerifiedOn     *string  `json:"verified_on"`
	LastSHA        *string  `json:"last_sha"`
	UpToDate       bool     `json:"up_to_date"`
	CommitsSince   int64    `json:"commits_since"`
	MissingNotes   []string `json:"missing_notes"` // titles listed in the card's expected_notes
	ActivityNote   string   `json:"activity_note"` // why the card is or is not dormant
}

type AIActivity struct {
	AI       string `json:"ai"`
	Sessions int64  `json:"sessions"`
	First    string `json:"first"`
	Last     string `json:"last"`
}

type ProjectFull struct {
	ProjectSummary
	SourcePath   *string  `json:"source_path"`
	Technologies []string `json:"technologies"`
}

type CommitSince struct {
	Date  string `json:"date"`
	Ref   string `json:"ref"`
	Title string `json:"title"`
	Since string `json:"since"` // latest session date these commits follow
}

type ProjectDetail struct {
	Project           ProjectFull     `json:"project"`
	Timeline          []TimelineEvent `json:"timeline"`
	Volume            []WeeklyVolume  `json:"volume"`
	Languages         []LanguageShare `json:"languages"`
	LanguageEvolution []LanguagePoint `json:"language_evolution"`
	Hotspots          []Hotspot       `json:"hotspots"`
	Health            Health          `json:"health"`
	AI                []AIActivity    `json:"ai"`
	SinceSession      []CommitSince   `json:"since_session"`
	RepoPaths         []string        `json:"repo_paths"`
	ObsidianURL       string          `json:"obsidian_url"`
}

// ---- GET /api/technologies and /api/ai

type SankeyNode struct {
	Name string `json:"name"`
	Type string `json:"type"` // project | technology
}

type SankeyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int64  `json:"value"` // 1 per project -> technology link
}

type Sankey struct {
	Nodes []SankeyNode `json:"nodes"`
	Links []SankeyLink `json:"links"`
}

type TechTrend struct {
	Month      string `json:"month"`
	Technology string `json:"technology"`
	Commits    int64  `json:"commits"`
}

type Technologies struct {
	Sankey     Sankey      `json:"sankey"`
	Trend      []TechTrend `json:"trend"`
	Unassigned []string    `json:"unassigned"`
}

type SessionsMonth struct {
	Month    string `json:"month"`
	Sessions int64  `json:"sessions"`
}

type Assistant struct {
	AI       string          `json:"ai"`
	Sessions int64           `json:"sessions"`
	Projects []string        `json:"projects"`
	ByMonth  []SessionsMonth `json:"by_month"`
}

// ---- GET /api/graph, /api/graph/path, /api/notes/{id}

type GraphNode struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Path   string `json:"path"`
	Folder string `json:"folder"`
	Degree int64  `json:"degree"`
}

type GraphLink struct {
	Source int64 `json:"source"`
	Target int64 `json:"target"`
}

type BrokenLink struct {
	Source      int64  `json:"source"`
	SourceTitle string `json:"source_title"`
	TargetTitle string `json:"target_title"`
}

type Graph struct {
	Nodes   []GraphNode  `json:"nodes"`
	Links   []GraphLink  `json:"links"`
	Orphans []GraphNode  `json:"orphans"`
	Central []GraphNode  `json:"central"`
	Broken  []BrokenLink `json:"broken"`
	Folders []string     `json:"folders"` // every folder, before the folder filter
}

type GraphPath struct {
	Path []GraphNode `json:"path"`
}

type NoteDetail struct {
	ID          int64       `json:"id"`
	Title       string      `json:"title"`
	Path        string      `json:"path"`
	Folder      string      `json:"folder"`
	Updated     *string     `json:"updated"`
	Content     string      `json:"content"`
	Neighbors   []GraphNode `json:"neighbors"`
	ObsidianURL string      `json:"obsidian_url"`
	Project     *ProjectRef `json:"project"` // project the note belongs to, or null
}

// ProjectRef names a project without its details.
type ProjectRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ---- GET /api/status

// Status describes what Vantage tracks and what the server can change.
type Status struct {
	Repositories int64  `json:"repositories"`
	Notes        int64  `json:"notes"`
	Vault        string `json:"vault"`          // vault name, empty when unknown
	CanLinkRepos bool   `json:"can_link_repos"` // true under "vantage watch"
}

// ---- POST /api/projects/{id}/repo

type LinkRepoRequest struct {
	Path string `json:"path"` // absolute folder of a Git repository
}

type LinkRepoResponse struct {
	SourcePath string `json:"source_path"`
}

// ---- GET /api/repos

type RepoFile struct {
	Path     string `json:"path"`
	Branch   string `json:"branch"`
	Exists   bool   `json:"exists"`
	Imported bool   `json:"imported"`
}

type RepoCard struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	SourcePath   string     `json:"source_path"`
	SourceExists bool       `json:"source_exists"`
	Branch       string     `json:"branch"`
	Repos        []RepoFile `json:"repos"`
}

// ---- GET /api/search

type SearchHit struct {
	Source  string  `json:"source"` // note | commit
	Ref     string  `json:"ref"`
	Title   string  `json:"title"`
	Excerpt string  `json:"excerpt"`
	Project *string `json:"project"`
	Date    *string `json:"date"`
	NoteID  *int64  `json:"note_id"`
}

// ---- SQL console

type SQLRequest struct {
	SQL string `json:"sql"`
}

type SQLResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Duration  float64  `json:"duration_ms"`
	Truncated bool     `json:"truncated"`
}

type SchemaColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type SchemaTable struct {
	Table   string         `json:"table"`
	Columns []SchemaColumn `json:"columns"`
}

type SavedQuery struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
}

// ---- GET /api/events (SSE), event "update"

type UpdateEvent struct {
	Tables []string `json:"tables"`
}

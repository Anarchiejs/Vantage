// JSON API contract. Exact mirror of internal/api/contract.go: every change is made on both sides.
// Dates: "YYYY-MM-DD"; instants: ISO 8601 UTC "2026-10-26T14:03:00Z"; months: "YYYY-MM"; weeks: Monday "YYYY-MM-DD".

export interface APIError {
  error: string
}

// ---- GET /api/home ----------------------------------------------------------

export interface ActivityDay {
  day: string
  commits: number
  additions: number
  deletions: number
  sessions: number
  projects: string[] // names of projects touched that day
}

export interface Stats {
  commits: number
  additions: number
  deletions: number
  sessions: number
  active_projects: number
}

export interface ActiveProject {
  id: number
  name: string
  last_commit: string | null // instant
  commits_30d: number
}

export interface DormantProject {
  id: number
  name: string
  last_commit: string | null
  days: number // since the last commit
}

export interface StaleCard {
  id: number
  name: string
  verified_commit: string | null
  last_sha: string | null
  commits_since: number // commits after verified_commit (-1 if verified_commit is unknown in the repository)
}

export interface Home {
  calendar: ActivityDay[] // 365 days ending today, including empty days, chronological
  week: Stats // last 7 days
  previous_week: Stats
  month: Stats // last 30 days
  previous_month: Stats
  year: Stats // last 365 days
  previous_year: Stats
  active: ActiveProject[]
  dormant: DormantProject[] // status "in progress" with no commit for more than 30 days
  stale_cards: StaleCard[]
  quiet_hidden: number // quiet cards hidden because status is outside the watched set
  recent_notes: RecentNote[] // six latest dated notes
}

export interface RecentNote {
  id: number
  title: string
  folder: string
  day: string // session date, or the note's updated date
}

// ---- GET /api/projects and /api/projects/{id} ------------------------------

export interface ProjectSummary {
  id: number
  name: string
  status: string | null
  commits: number
  last_commit: string | null
  primary_language: string | null
}

export type EventType = 'commit' | 'session' | 'journal'

export interface TimelineEvent {
  date: string // instant for a commit, date otherwise
  type: EventType
  title: string // commit subject, session title, or journal text
  ref: string // sha, note path, or journal id
  ai: string | null
}

export interface WeeklyVolume {
  week: string
  commits: number
  additions: number
  deletions: number
}

export interface LanguageShare {
  language: string
  code_lines: number
  share: number // 0..1
}

export interface LanguagePoint {
  measured_on: string
  language: string
  code_lines: number
}

export interface Hotspot {
  path: string
  modifications: number
  additions: number
  deletions: number
}

export interface Health {
  verified_commit: string | null
  verified_on: string | null
  last_sha: string | null
  up_to_date: boolean
  commits_since: number
  missing_notes: string[] // titles listed in the card's expected_notes
  activity_note: string // why the card is or is not dormant
}

export interface AIActivity {
  ai: string
  sessions: number
  first: string
  last: string
}

export interface CommitSince {
  date: string
  ref: string
  title: string
  since: string // latest session date these commits follow
}

export interface ProjectDetail {
  project: ProjectSummary & { source_path: string | null; technologies: string[] }
  timeline: TimelineEvent[] // newest first, at most 500
  volume: WeeklyVolume[] // last 52 weeks, including empty weeks
  languages: LanguageShare[] // latest measurement
  language_evolution: LanguagePoint[]
  hotspots: Hotspot[] // 20 most modified files
  health: Health
  ai: AIActivity[]
  since_session: CommitSince[]
  repo_paths: string[]
  obsidian_url: string
}

// ---- GET /api/technologies and /api/ai -------------------------------------

export interface SankeyNode {
  name: string
  type: 'project' | 'technology'
}

export interface SankeyLink {
  source: string
  target: string
  value: number // 1 per project -> technology link
}

export interface TechTrend {
  month: string
  technology: string
  commits: number // commits of projects that use the technology, that month
}

export interface Technologies {
  sankey: { nodes: SankeyNode[]; links: SankeyLink[] }
  trend: TechTrend[] // last 12 months
  unassigned: string[]
}

export interface Assistant {
  ai: string
  sessions: number
  projects: string[]
  by_month: { month: string; sessions: number }[]
}

// ---- GET /api/graph, /api/graph/path, /api/notes/{id} ----------------------

export interface GraphNode {
  id: number
  title: string
  path: string
  folder: string
  degree: number // resolved incoming + outgoing links
}

export interface Graph {
  nodes: GraphNode[]
  links: { source: number; target: number }[]
  orphans: GraphNode[] // degree = 0
  central: GraphNode[] // 10 highest degrees
  broken: { source: number; source_title: string; target_title: string }[]
  folders: string[] // every folder, before the folder filter
}

export interface GraphPath {
  path: GraphNode[] // empty when there is no path; links are followed in both directions
}

export interface NoteDetail {
  id: number
  title: string
  path: string
  folder: string
  updated: string | null
  content: string
  neighbors: GraphNode[]
  obsidian_url: string
  project: ProjectRef | null // project the note belongs to
}

export interface ProjectRef {
  id: number
  name: string
}

// ---- GET /api/status and POST /api/projects/{id}/repo ----------------------

export interface Status {
  repositories: number
  notes: number
  vault: string // vault name, empty when unknown
  can_link_repos: boolean // true under "vantage watch"
}

export interface LinkRepoRequest {
  path: string // absolute folder of a Git repository
}

export interface LinkRepoResponse {
  source_path: string
}

export interface RepoFile {
  path: string
  branch: string
  exists: boolean
  imported: boolean
}

export interface RepoCard {
  id: number
  name: string
  source_path: string
  source_exists: boolean
  branch: string
  repos: RepoFile[]
}

// ---- GET /api/search ---------------------------------------------------------

export interface SearchHit {
  source: 'note' | 'commit'
  ref: string // note path or sha
  title: string // matched terms wrapped in U+E000 / U+E001
  excerpt: string // same, with "…" at cuts
  project: string | null
  date: string | null
  note_id: number | null
}

// Parameters: q (required), source, project, folder, since, until, limit (default 50)

// ---- SQL console -------------------------------------------------------------

export interface SQLRequest {
  sql: string
}

export interface SQLResult {
  columns: string[]
  rows: (string | number | null)[][]
  duration_ms: number
  truncated: boolean // more than 1000 rows: only the first 1000 are returned
}

export interface SchemaTable {
  table: string
  columns: { name: string; type: string }[]
}

export interface SavedQuery {
  name: string // file name without ".sql", [a-z0-9-] only
  sql: string
}

// ---- GET /api/events (Server-Sent Events) ----------------------------------
// event "update": { "tables": ["commit_git", "note", ...] } after each import.

export interface UpdateEvent {
  tables: string[]
}

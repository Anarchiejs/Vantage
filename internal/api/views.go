package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Anarchiejs/Vantage/internal/sql/gen"
)

// viewRoutes wires the UI views to the queries in internal/sql/queries.
func (s *Server) viewRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/home", s.home)
	mux.HandleFunc("GET /api/projects", s.projects)
	mux.HandleFunc("GET /api/projects/{id}", s.project)
	mux.HandleFunc("GET /api/technologies", s.technologies)
	mux.HandleFunc("GET /api/ai", s.assistants)
	mux.HandleFunc("GET /api/graph", s.graph)
	mux.HandleFunc("GET /api/graph/path", s.path)
	mux.HandleFunc("GET /api/notes/{id}", s.note)
	mux.HandleFunc("GET /api/search", s.search)
	mux.HandleFunc("GET /api/repos", s.repos)
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("POST /api/projects/{id}/repo", s.linkRepo)
}

// failed writes 500 (or 404 for a missing row) and reports whether the handler should stop.
func failed(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
	} else {
		writeError(w, http.StatusInternalServerError, err.Error())
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// text converts computed values that sqlc types as interface{} (text or NULL).
func text(v any) *string {
	switch x := v.(type) {
	case string:
		return &x
	case []byte:
		s := string(x)
		return &s
	case nil:
		return nil
	default:
		s := fmt.Sprint(x)
		return &s
	}
}

func textOrEmpty(v any) string {
	if p := text(v); p != nil {
		return *p
	}
	return ""
}

func shiftDay(day string, days int) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

func (s *Server) stats(r *http.Request, start, end string) (Stats, error) {
	row, err := s.q.PeriodStats(r.Context(), gen.PeriodStatsParams{StartDate: start, EndDate: end})
	return Stats{Commits: row.Commits, Additions: row.Additions, Deletions: row.Deletions, Sessions: row.Sessions, ActiveProjects: row.ActiveProjects}, err
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	today := s.cfg.Today()
	res := Home{Calendar: []ActivityDay{}, Active: []ActiveProject{}, Dormant: []DormantProject{}, StaleCards: []StaleCard{}, RecentNotes: []RecentNote{}}

	calendar, err := s.q.Calendar(ctx, today)
	if failed(w, err) {
		return
	}
	for _, day := range calendar {
		projects := []string{}
		if day.Projects != "" {
			projects = strings.Split(day.Projects, "\x1f")
		}
		res.Calendar = append(res.Calendar, ActivityDay{Day: day.Day, Commits: day.Commits, Additions: day.Additions, Deletions: day.Deletions, Sessions: day.Sessions, Projects: projects})
	}

	periods := []struct {
		dest       *Stats
		start, end int
	}{
		{&res.Week, -6, 0}, {&res.PreviousWeek, -13, -7}, {&res.Month, -29, 0}, {&res.PreviousMonth, -59, -30},
		{&res.Year, -364, 0}, {&res.PreviousYear, -729, -365},
	}
	for _, period := range periods {
		if *period.dest, err = s.stats(r, shiftDay(today, period.start), shiftDay(today, period.end)); failed(w, err) {
			return
		}
	}

	active, err := s.q.ActiveProjects(ctx, today)
	if failed(w, err) {
		return
	}
	for _, row := range active {
		res.Active = append(res.Active, ActiveProject{ID: row.ID, Name: row.Name, LastCommit: text(row.LastCommit), Commits30d: row.Commits30d})
	}
	dormant, err := s.q.DormantProjects(ctx, today)
	if failed(w, err) {
		return
	}
	for _, row := range dormant {
		res.Dormant = append(res.Dormant, DormantProject{ID: row.ID, Name: row.Name, LastCommit: text(row.LastCommit), Days: row.Days})
	}
	stale, err := s.q.StaleCards(ctx)
	if failed(w, err) {
		return
	}
	for _, row := range stale {
		res.StaleCards = append(res.StaleCards, StaleCard{ID: row.ID, Name: row.Name, VerifiedCommit: row.VerifiedCommit, LastSHA: row.LastSha, CommitsSince: row.CommitsSince})
	}
	quiet, err := s.q.QuietOutsideStatus(ctx, &today)
	if failed(w, err) {
		return
	}
	res.QuietHidden = quiet
	recent, err := s.q.RecentNotes(ctx)
	if failed(w, err) {
		return
	}
	for _, row := range recent {
		res.RecentNotes = append(res.RecentNotes, RecentNote{ID: row.ID, Title: row.Title, Folder: row.Folder, Day: row.Day})
	}
	writeJSON(w, res)
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListProjects(r.Context())
	if failed(w, err) {
		return
	}
	res := make([]ProjectSummary, 0, len(rows))
	for _, row := range rows {
		res = append(res, ProjectSummary{ID: row.ID, Name: row.Name, Status: row.Status, Commits: row.Commits, LastCommit: text(row.LastCommit), PrimaryLanguage: row.PrimaryLanguage})
	}
	writeJSON(w, res)
}

func (s *Server) project(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	row, err := s.q.ProjectByID(ctx, id)
	if failed(w, err) {
		return
	}
	res := ProjectDetail{Timeline: []TimelineEvent{}, Volume: []WeeklyVolume{}, Languages: []LanguageShare{}, LanguageEvolution: []LanguagePoint{}, Hotspots: []Hotspot{}, AI: []AIActivity{}, SinceSession: []CommitSince{}, RepoPaths: []string{}}

	// Summary reuses the list row (commits, last commit, primary language).
	res.Project = ProjectFull{ProjectSummary: ProjectSummary{ID: row.ID, Name: row.Name, Status: row.Status}, SourcePath: row.SourcePath}
	list, err := s.q.ListProjects(ctx)
	if failed(w, err) {
		return
	}
	for _, item := range list {
		if item.ID == id {
			res.Project.Commits, res.Project.LastCommit, res.Project.PrimaryLanguage = item.Commits, text(item.LastCommit), item.PrimaryLanguage
		}
	}
	if res.Project.Technologies, err = s.q.ProjectTechnologies(ctx, id); failed(w, err) {
		return
	}
	if res.Project.Technologies == nil {
		res.Project.Technologies = []string{}
	}

	timeline, err := s.q.Timeline(ctx, id)
	if failed(w, err) {
		return
	}
	for _, event := range timeline {
		res.Timeline = append(res.Timeline, TimelineEvent{Date: event.Date, Type: event.Type, Title: textOrEmpty(event.Title), Ref: event.Ref, AI: event.Ai})
	}
	volume, err := s.q.WeeklyVolume(ctx, gen.WeeklyVolumeParams{ProjectID: id, Today: s.cfg.Today()})
	if failed(w, err) {
		return
	}
	for _, week := range volume {
		res.Volume = append(res.Volume, WeeklyVolume{Week: week.Week, Commits: week.Commits, Additions: week.Additions, Deletions: week.Deletions})
	}
	languages, err := s.q.LatestLanguages(ctx, id)
	if failed(w, err) {
		return
	}
	for _, language := range languages {
		res.Languages = append(res.Languages, LanguageShare{Language: language.Language, CodeLines: language.CodeLines, Share: language.Share})
	}
	evolution, err := s.q.LanguageEvolution(ctx, id)
	if failed(w, err) {
		return
	}
	for _, language := range evolution {
		res.LanguageEvolution = append(res.LanguageEvolution, LanguagePoint{MeasuredOn: language.MeasuredOn, Language: language.Language, CodeLines: language.CodeLines})
	}
	hotspots, err := s.q.Hotspots(ctx, id)
	if failed(w, err) {
		return
	}
	for _, hotspot := range hotspots {
		res.Hotspots = append(res.Hotspots, Hotspot{Path: hotspot.Path, Modifications: hotspot.Modifications, Additions: hotspot.Additions, Deletions: hotspot.Deletions})
	}
	assistants, err := s.q.ProjectAI(ctx, &id)
	if failed(w, err) {
		return
	}
	for _, assistant := range assistants {
		if assistant.Ai == nil {
			continue
		}
		res.AI = append(res.AI, AIActivity{AI: *assistant.Ai, Sessions: assistant.Sessions, First: textOrEmpty(assistant.FirstOn), Last: textOrEmpty(assistant.LastOn)})
	}

	if res.Health, err = s.health(r, row); failed(w, err) {
		return
	}
	repositories, err := s.q.ProjectRepositories(ctx, id)
	if failed(w, err) {
		return
	}
	for _, repo := range repositories {
		res.RepoPaths = append(res.RepoPaths, repo.Path)
	}
	since, err := s.q.CommitsSinceSession(ctx, &id)
	if failed(w, err) {
		return
	}
	for _, commit := range since {
		res.SinceSession = append(res.SinceSession, CommitSince{Date: commit.Date, Ref: commit.Ref, Title: textOrEmpty(commit.Title), Since: textOrEmpty(commit.Since)})
	}
	res.ObsidianURL = s.obsidianURL(ctx, filepath.ToSlash(filepath.Join("Projects", row.Name, row.Name+".md")))
	writeJSON(w, res)
}

func (s *Server) health(r *http.Request, project gen.Project) (Health, error) {
	ctx := r.Context()
	health := Health{VerifiedCommit: project.VerifiedCommit, VerifiedOn: project.VerifiedOn, UpToDate: true, MissingNotes: []string{}}
	repositories, err := s.q.ProjectRepositories(ctx, project.ID)
	if err != nil {
		return health, err
	}
	if len(repositories) > 0 {
		health.LastSHA = repositories[0].LastImportedSha
	}
	stale, err := s.q.StaleCards(ctx)
	if err != nil {
		return health, err
	}
	for _, card := range stale {
		if card.ID == project.ID {
			health.UpToDate = false
			health.CommitsSince = max(health.CommitsSince, card.CommitsSince)
			if card.CommitsSince < 0 {
				health.CommitsSince = card.CommitsSince
			}
		}
	}
	present, err := s.q.ProjectNotes(ctx, project.ID)
	if err != nil {
		return health, err
	}
	var expected []string
	if project.ExpectedNotes != nil && *project.ExpectedNotes != "" {
		if err := json.Unmarshal([]byte(*project.ExpectedNotes), &expected); err != nil {
			return health, err
		}
	}
	for _, name := range expected {
		found := false
		for _, title := range present {
			found = found || title == name
		}
		if !found {
			health.MissingNotes = append(health.MissingNotes, name)
		}
	}
	status := ""
	if project.Status != nil {
		status = strings.ToLower(strings.TrimSpace(*project.Status))
	}
	watched := status == "in progress" || status == "active" || status == "ongoing"
	dormant, err := s.q.DormantProjects(ctx, s.cfg.Today())
	if err != nil {
		return health, err
	}
	listed := false
	for _, card := range dormant {
		if card.ID == project.ID {
			listed = true
		}
	}
	switch {
	case !watched:
		shown := "empty"
		if project.Status != nil && strings.TrimSpace(*project.Status) != "" {
			shown = *project.Status
		}
		health.ActivityNote = fmt.Sprintf("Not dormant: status %q is outside in progress, active, and ongoing.", shown)
	case listed:
		health.ActivityNote = "Dormant: status is in progress, active, or ongoing, and the last commit is missing or older than 30 days."
	default:
		health.ActivityNote = "Not dormant: status is watched and the last commit is within 30 days."
	}
	return health, nil
}

func (s *Server) technologies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	res := Technologies{Sankey: Sankey{Nodes: []SankeyNode{}, Links: []SankeyLink{}}, Trend: []TechTrend{}}
	links, err := s.q.SankeyLinks(ctx)
	if failed(w, err) {
		return
	}
	// ECharts requires unique node names: a technology that shares a project's name is suffixed.
	projects := map[string]bool{}
	for _, link := range links {
		projects[link.Source] = true
	}
	seen := map[string]bool{}
	for _, link := range links {
		target := link.Target
		if projects[target] {
			target += " (tech)"
		}
		for _, node := range []SankeyNode{{link.Source, "project"}, {target, "technology"}} {
			if !seen[node.Name] {
				seen[node.Name] = true
				res.Sankey.Nodes = append(res.Sankey.Nodes, node)
			}
		}
		res.Sankey.Links = append(res.Sankey.Links, SankeyLink{Source: link.Source, Target: target, Value: link.Value})
	}
	trend, err := s.q.TechnologyTrend(ctx, s.cfg.Today())
	if failed(w, err) {
		return
	}
	for _, row := range trend {
		res.Trend = append(res.Trend, TechTrend{Month: row.Month, Technology: row.Technology, Commits: row.Commits})
	}
	if res.Unassigned, err = s.q.UnassignedTechnologies(ctx); failed(w, err) {
		return
	}
	if res.Unassigned == nil {
		res.Unassigned = []string{}
	}
	writeJSON(w, res)
}

func (s *Server) assistants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	activity, err := s.q.AIActivity(ctx)
	if failed(w, err) {
		return
	}
	byAI := map[string]*Assistant{}
	var order []string
	for _, row := range activity {
		if row.Ai == nil {
			continue
		}
		byAI[*row.Ai] = &Assistant{AI: *row.Ai, Sessions: row.Sessions, Projects: []string{}, ByMonth: []SessionsMonth{}}
		order = append(order, *row.Ai)
	}
	projects, err := s.q.ProjectsByAI(ctx)
	if failed(w, err) {
		return
	}
	for _, project := range projects {
		if project.Ai != nil && byAI[*project.Ai] != nil {
			byAI[*project.Ai].Projects = append(byAI[*project.Ai].Projects, project.Name)
		}
	}
	months, err := s.q.AISessionsByMonth(ctx, s.cfg.Today())
	if failed(w, err) {
		return
	}
	for _, month := range months {
		if month.Ai != nil && byAI[*month.Ai] != nil {
			byAI[*month.Ai].ByMonth = append(byAI[*month.Ai].ByMonth, SessionsMonth{Month: month.Month, Sessions: month.Sessions})
		}
	}
	res := make([]Assistant, 0, len(order))
	for _, name := range order {
		res = append(res, *byAI[name])
	}
	writeJSON(w, res)
}

func graphNode(id int64, title, path, folder string, degree int64) GraphNode {
	return GraphNode{ID: id, Title: title, Path: path, Folder: folder, Degree: degree}
}

func (s *Server) graph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	res := Graph{Nodes: []GraphNode{}, Links: []GraphLink{}, Orphans: []GraphNode{}, Central: []GraphNode{}, Broken: []BrokenLink{}, Folders: []string{}}
	nodes, err := s.q.GraphNodes(ctx)
	if failed(w, err) {
		return
	}
	folderSet := map[string]bool{}
	for _, node := range nodes {
		if node.Folder != "" {
			folderSet[node.Folder] = true
		}
	}
	for folder := range folderSet {
		res.Folders = append(res.Folders, folder)
	}
	sort.Strings(res.Folders)
	wanted := map[string]bool{}
	for _, folder := range r.URL.Query()["folder"] {
		if folder != "" {
			wanted[folder] = true
		}
	}
	includeProperties := r.URL.Query().Get("properties") != "0"
	keep := map[int64]GraphNode{}
	for _, node := range nodes {
		if len(wanted) > 0 && !wanted[node.Folder] {
			continue
		}
		keep[node.ID] = graphNode(node.ID, node.Title, node.Path, node.Folder, 0)
	}
	links, err := s.q.GraphLinks(ctx)
	if failed(w, err) {
		return
	}
	degree := map[int64]int64{}
	for _, link := range links {
		if link.Target == nil || !includeProperties && link.FromProperty != 0 {
			continue
		}
		if _, ok := keep[link.Source]; !ok {
			continue
		}
		if _, ok := keep[*link.Target]; !ok {
			continue
		}
		res.Links = append(res.Links, GraphLink{Source: link.Source, Target: *link.Target})
		degree[link.Source]++
		degree[*link.Target]++
	}
	for id, node := range keep {
		node.Degree = degree[id]
		keep[id] = node
		res.Nodes = append(res.Nodes, node)
	}
	sort.Slice(res.Nodes, func(i, j int) bool { return res.Nodes[i].ID < res.Nodes[j].ID })
	for _, node := range res.Nodes {
		if node.Degree == 0 {
			res.Orphans = append(res.Orphans, node)
		}
	}
	central := append([]GraphNode(nil), res.Nodes...)
	sort.SliceStable(central, func(i, j int) bool { return central[i].Degree > central[j].Degree })
	for _, node := range central[:min(10, len(central))] {
		if node.Degree > 0 {
			res.Central = append(res.Central, node)
		}
	}
	broken, err := s.q.BrokenLinks(ctx)
	if failed(w, err) {
		return
	}
	for _, link := range broken {
		if !includeProperties && link.FromProperty != 0 {
			continue
		}
		node, ok := keep[link.Source]
		if !ok {
			continue
		}
		res.Broken = append(res.Broken, BrokenLink{Source: link.Source, SourceTitle: node.Title, TargetTitle: link.TargetTitle})
	}
	writeJSON(w, res)
}

// path finds the shortest path with a breadth-first search in Go: linear in notes and links,
// where a SQL recursion over simple paths explodes on a dense graph. Links are followed in
// both directions; at equal length, the neighbor with the smaller id comes first.
func (s *Server) path(w http.ResponseWriter, r *http.Request) {
	from, err1 := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	to, err2 := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "parameters from and to: note ids expected")
		return
	}
	ctx := r.Context()
	nodes, err := s.q.GraphNodes(ctx)
	if failed(w, err) {
		return
	}
	byID := make(map[int64]GraphNode, len(nodes))
	for _, node := range nodes {
		byID[node.ID] = graphNode(node.ID, node.Title, node.Path, node.Folder, node.Degree)
	}
	if _, ok := byID[from]; !ok {
		writeError(w, http.StatusNotFound, "start note not found")
		return
	}
	if _, ok := byID[to]; !ok {
		writeError(w, http.StatusNotFound, "end note not found")
		return
	}
	links, err := s.q.GraphLinks(ctx)
	if failed(w, err) {
		return
	}
	neighbors := map[int64][]int64{}
	for _, link := range links {
		if link.Target != nil && *link.Target != link.Source {
			neighbors[link.Source] = append(neighbors[link.Source], *link.Target)
			neighbors[*link.Target] = append(neighbors[*link.Target], link.Source)
		}
	}
	for _, list := range neighbors {
		slices.Sort(list)
	}

	res := GraphPath{Path: []GraphNode{}}
	previous := map[int64]int64{from: from}
	queue := []int64{from}
	for len(queue) > 0 {
		if _, found := previous[to]; found {
			break
		}
		current := queue[0]
		queue = queue[1:]
		for _, next := range neighbors[current] {
			if _, seen := previous[next]; !seen {
				previous[next] = current
				queue = append(queue, next)
			}
		}
	}
	if _, found := previous[to]; found {
		for n := to; ; n = previous[n] {
			res.Path = append(res.Path, byID[n])
			if n == from {
				break
			}
		}
		slices.Reverse(res.Path)
	}
	writeJSON(w, res)
}

func (s *Server) note(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	row, err := s.q.NoteByID(r.Context(), id)
	if failed(w, err) {
		return
	}
	neighbors, err := s.q.Neighbors(r.Context(), id)
	if failed(w, err) {
		return
	}
	res := NoteDetail{ID: row.ID, Title: row.Title, Path: row.Path, Folder: row.Folder, Updated: row.Updated, Content: row.Content, Neighbors: []GraphNode{}}
	for _, neighbor := range neighbors {
		res.Neighbors = append(res.Neighbors, graphNode(neighbor.ID, neighbor.Title, neighbor.Path, neighbor.Folder, neighbor.Degree))
	}
	res.ObsidianURL = s.obsidianURL(r.Context(), row.Path)
	owners, err := s.q.NoteProject(r.Context(), id)
	if failed(w, err) {
		return
	}
	if len(owners) > 0 {
		res.Project = &ProjectRef{ID: owners[0].ID, Name: owners[0].Name}
	}
	writeJSON(w, res)
}

func (s *Server) obsidianURL(ctx context.Context, file string) string {
	name, err := s.q.Setting(ctx, "vault_name")
	if err != nil || name == "" {
		return ""
	}
	return "obsidian://open?vault=" + url.QueryEscape(name) + "&file=" + url.QueryEscape(file)
}

func (s *Server) repos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.q.ListProjects(ctx)
	if failed(w, err) {
		return
	}
	res := make([]RepoCard, 0, len(rows))
	for _, row := range rows {
		project, err := s.q.ProjectByID(ctx, row.ID)
		if failed(w, err) {
			return
		}
		card := RepoCard{ID: row.ID, Name: row.Name, Repos: []RepoFile{}}
		if project.SourcePath != nil {
			card.SourcePath = *project.SourcePath
			if info, err := os.Stat(card.SourcePath); err == nil && info.IsDir() {
				card.SourceExists = true
			}
		}
		if project.Branch != nil {
			card.Branch = *project.Branch
		}
		repositories, err := s.q.ProjectRepositories(ctx, row.ID)
		if failed(w, err) {
			return
		}
		for _, repo := range repositories {
			file := RepoFile{Path: repo.Path, Imported: true}
			if repo.Branch != nil {
				file.Branch = *repo.Branch
			}
			if info, err := os.Stat(repo.Path); err == nil && info.IsDir() {
				file.Exists = true
			}
			card.Repos = append(card.Repos, file)
		}
		res = append(res, card)
	}
	writeJSON(w, res)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	terms := strings.TrimSpace(query.Get("q"))
	if terms == "" {
		writeJSON(w, []SearchHit{})
		return
	}
	limit := int64(50)
	if n, err := strconv.ParseInt(query.Get("limit"), 10, 64); err == nil && n > 0 {
		limit = min(n, 200)
	}
	optional := func(name string) any {
		if v := strings.TrimSpace(query.Get(name)); v != "" {
			return v
		}
		return nil
	}
	rows, err := s.q.Search(r.Context(), gen.SearchParams{
		Terms: terms, Source: optional("source"), Project: optional("project"), Folder: optional("folder"),
		Since: optional("since"), Until: optional("until"), RowLimit: limit,
	})
	if err != nil {
		// Invalid FTS5 syntax (unclosed quote, lone operator) is a user error.
		writeError(w, http.StatusBadRequest, "invalid search: "+err.Error())
		return
	}
	res := make([]SearchHit, 0, len(rows))
	for _, row := range rows {
		res = append(res, SearchHit{Source: row.Source, Ref: row.Ref, Title: row.Title, Excerpt: row.Excerpt, Project: row.Project, Date: text(row.Date), NoteID: row.NoteID})
	}
	writeJSON(w, res)
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Anarchiejs/Vantage/internal/db"
	"github.com/Anarchiejs/Vantage/internal/ingest"
)

func repoRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

// serverFixtures: database filled by importing the fixtures, with "today" fixed at 2026-03-31.
func serveurFixtures(t *testing.T) http.Handler {
	t.Helper()
	r := repoRoot()
	path := filepath.Join(t.TempDir(), "test.db")
	rw, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rw.Close() })
	if err := db.Migrate(rw); err != nil {
		t.Fatal(err)
	}
	b, err := ingest.Import(context.Background(), rw, ingest.Options{
		Vault: filepath.Join(r, "testdata", "vault"),
		Repos: map[string]string{"Atlas": filepath.Join(r, "testdata", "repos", "alpha"), "Compass": filepath.Join(r, "testdata", "repos", "beta")},
		Today: "2026-03-31",
	})
	if err != nil || len(b.Warnings) > 0 {
		t.Fatalf("import : %v %v", err, b.Warnings)
	}
	ro, err := db.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ro.Close() })
	s, err := New(Config{Address: "127.0.0.1:0", DB: rw, ReadDB: ro, Today: func() string { return "2026-03-31" }})
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

// read calls the route and decodes strictly: a field outside the contract fails the test.
func getJSON[T any](t *testing.T, h http.Handler, url string, wantStatus int) T {
	t.Helper()
	w := callHandler(h, "GET", url, "127.0.0.1", "")
	if w.Code != wantStatus {
		t.Fatalf("%s : %d %s", url, w.Code, w.Body)
	}
	var v T
	dec := json.NewDecoder(w.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("%s : %v", url, err)
	}
	return v
}

func noNilList(t *testing.T, url string, v any) {
	t.Helper()
	var visit func(path string, x reflect.Value)
	visit = func(path string, x reflect.Value) {
		switch x.Kind() {
		case reflect.Pointer:
			if !x.IsNil() {
				visit(path, x.Elem())
			}
		case reflect.Struct:
			for i := 0; i < x.NumField(); i++ {
				visit(path+"."+x.Type().Field(i).Name, x.Field(i))
			}
		case reflect.Slice:
			if x.IsNil() {
				t.Errorf("%s : nil list %s (must be [])", url, path)
			}
			for i := 0; i < x.Len(); i++ {
				visit(path, x.Index(i))
			}
		}
	}
	visit("", reflect.ValueOf(v))
}

func TestViews(t *testing.T) {
	h := serveurFixtures(t)

	acc := getJSON[Home](t, h, "/api/home", 200)
	noNilList(t, "home", acc)
	if len(acc.Calendar) != 365 || acc.Calendar[364].Day != "2026-03-31" {
		t.Fatalf("calendar: %d days, last %v", len(acc.Calendar), acc.Calendar[len(acc.Calendar)-1])
	}
	var total int64
	for _, j := range acc.Calendar {
		total += j.Commits
	}
	if total != 42 {
		t.Errorf("calendar: %d commits, expected 42", total)
	}
	if acc.Month.Commits != 12 { // 6 commits in March per repository (see internal/sql/queries/README.md)
		t.Errorf("month: %d commits", acc.Month.Commits)
	}

	projects := getJSON[[]ProjectSummary](t, h, "/api/projects", 200)
	if len(projects) < 2 {
		t.Fatalf("projects : %+v", projects)
	}
	var atlas int64
	for _, p := range projects {
		if p.Name == "Atlas" {
			atlas = p.ID
		}
	}
	detail := getJSON[ProjectDetail](t, h, "/api/projects/"+itoa(atlas), 200)
	noNilList(t, "project", detail)
	if detail.Project.Commits != 21 || len(detail.Volume) != 52 || len(detail.Languages) == 0 || len(detail.Project.Technologies) == 0 {
		t.Errorf("project Atlas: commits=%d weeks=%d languages=%d technologies=%v", detail.Project.Commits, len(detail.Volume), len(detail.Languages), detail.Project.Technologies)
	}
	if detail.Health.LastSHA == nil || detail.Health.UpToDate {
		t.Errorf("Atlas health (fictional verified_commit a1b2c3d) : %+v", detail.Health)
	}
	getJSON[APIError](t, h, "/api/projects/999999", 404)
	getJSON[APIError](t, h, "/api/projects/abc", 400)

	tech := getJSON[Technologies](t, h, "/api/technologies", 200)
	noNilList(t, "technologies", tech)
	noms := map[string]bool{}
	for _, n := range tech.Sankey.Nodes {
		if noms[n.Name] {
			t.Errorf("duplicate Sankey node : %s", n.Name)
		}
		noms[n.Name] = true
	}
	for _, l := range tech.Sankey.Links {
		if !noms[l.Source] || !noms[l.Target] {
			t.Errorf("Sankey link to a missing node : %+v", l)
		}
	}
	assistants := getJSON[[]Assistant](t, h, "/api/ai", 200)
	noNilList(t, "ai", assistants)

	g := getJSON[Graph](t, h, "/api/graph", 200)
	noNilList(t, "graph", g)
	if len(g.Nodes) == 0 || len(g.Links) == 0 || len(g.Broken) != 6 {
		t.Errorf("graph: %d nodes, %d links, %d broken", len(g.Nodes), len(g.Links), len(g.Broken))
	}
	n := getJSON[NoteDetail](t, h, "/api/notes/"+itoa(g.Nodes[0].ID), 200)
	noNilList(t, "note", n)
	pathResult := getJSON[GraphPath](t, h, "/api/graph/path?from="+itoa(g.Links[0].Source)+"&to="+itoa(g.Links[0].Target), 200)
	if len(pathResult.Path) != 2 {
		t.Errorf("direct path: %+v", pathResult)
	}
	getJSON[APIError](t, h, "/api/graph/path?from=x", 400)

	rech := getJSON[[]SearchHit](t, h, "/api/search?q=naive", 200)
	if len(rech) == 0 || !strings.Contains(rech[0].Title+rech[0].Excerpt, "\uE000") {
		t.Errorf("recherche : %+v", rech)
	}
	if v := getJSON[[]SearchHit](t, h, "/api/search?q=", 200); len(v) != 0 {
		t.Errorf("empty search: %+v", v)
	}
	getJSON[APIError](t, h, `/api/search?q=%22unclosed`, 400)
	getJSON[[]SearchHit](t, h, "/api/search?q=fixture&source=commit&limit=5", 200)
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// TestContractMirror checks that every interface in web/src/api/types.ts has exactly the JSON fields
// of the Go struct with the same name (contract.go).
func TestContractMirror(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(), "web", "src", "api", "types.ts"))
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile(`(?s)export interface (\w+)(?: extends [^{]+)? \{(.*?)\n\}`).FindAllStringSubmatch(string(src), -1)
	fieldTS := regexp.MustCompile(`(?m)^  (\w+)\??:`)
	types := map[string]reflect.Type{}
	for _, v := range []any{APIError{}, ActivityDay{}, Stats{}, ActiveProject{}, DormantProject{}, StaleCard{}, Home{},
		ProjectSummary{}, TimelineEvent{}, WeeklyVolume{}, LanguageShare{}, LanguagePoint{}, Hotspot{}, Health{}, AIActivity{},
		CommitSince{}, ProjectDetail{}, SankeyNode{}, SankeyLink{}, TechTrend{}, Technologies{}, Assistant{}, GraphNode{}, Graph{}, GraphPath{},
		NoteDetail{}, RepoFile{}, RepoCard{}, SearchHit{}, SQLRequest{}, SQLResult{}, SchemaTable{}, SavedQuery{}, UpdateEvent{},
		RecentNote{}, ProjectRef{}, Status{}, LinkRepoRequest{}, LinkRepoResponse{}} {
		types[reflect.TypeOf(v).Name()] = reflect.TypeOf(v)
	}
	if len(blocks) < 25 {
		t.Fatalf("only %d interfaces read in types.ts", len(blocks))
	}
	for _, b := range blocks {
		typ, ok := types[b[1]]
		if !ok {
			t.Errorf("TypeScript interface %s has no Go struct", b[1])
			continue
		}
		var ts []string
		for _, m := range fieldTS.FindAllStringSubmatch(b[2], -1) {
			ts = append(ts, m[1])
		}
		var goFields []string
		for i := 0; i < typ.NumField(); i++ {
			if tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]; tag != "" {
				goFields = append(goFields, tag)
			}
		}
		sort.Strings(ts)
		sort.Strings(goFields)
		if !reflect.DeepEqual(ts, goFields) {
			t.Errorf("%s : TS %v ≠ Go %v", b[1], ts, goFields)
		}
	}
}

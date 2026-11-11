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

	"github.com/Anarchiejs/Observatoire/internal/db"
	"github.com/Anarchiejs/Observatoire/internal/ingest"
)

func racine() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

// serverFixtures: database filled by importing the fixtures, with "today" fixed at 2026-03-31.
func serveurFixtures(t *testing.T) http.Handler {
	t.Helper()
	r := racine()
	if _, err := os.Stat(filepath.Join(r, "testdata", "depots", "alpha", ".git")); err != nil {
		t.Fatal("fixtures missing: run go run ./scripts/fixtures/make-repos.go first")
	}
	chemin := filepath.Join(t.TempDir(), "test.db")
	rw, err := db.Open(chemin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rw.Close() })
	if err := db.Migrate(rw); err != nil {
		t.Fatal(err)
	}
	b, err := ingest.Import(context.Background(), rw, ingest.Options{
		Vault: filepath.Join(r, "testdata", "vault"),
		Repos: map[string]string{"Atlas": filepath.Join(r, "testdata", "depots", "alpha"), "Compass": filepath.Join(r, "testdata", "depots", "beta")},
		Today: "2026-03-31",
	})
	if err != nil || len(b.Warnings) > 0 {
		t.Fatalf("import : %v %v", err, b.Warnings)
	}
	ro, err := db.OpenReadOnly(chemin)
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
func lire[T any](t *testing.T, h http.Handler, url string, statut int) T {
	t.Helper()
	w := appeler(h, "GET", url, "127.0.0.1", "")
	if w.Code != statut {
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

func pasDeListeNulle(t *testing.T, url string, v any) {
	t.Helper()
	var visiter func(chemin string, x reflect.Value)
	visiter = func(chemin string, x reflect.Value) {
		switch x.Kind() {
		case reflect.Pointer:
			if !x.IsNil() {
				visiter(chemin, x.Elem())
			}
		case reflect.Struct:
			for i := 0; i < x.NumField(); i++ {
				visiter(chemin+"."+x.Type().Field(i).Name, x.Field(i))
			}
		case reflect.Slice:
			if x.IsNil() {
				t.Errorf("%s : nil list %s (must be [])", url, chemin)
			}
			for i := 0; i < x.Len(); i++ {
				visiter(chemin, x.Index(i))
			}
		}
	}
	visiter("", reflect.ValueOf(v))
}

func TestVues(t *testing.T) {
	h := serveurFixtures(t)

	acc := lire[Home](t, h, "/api/home", 200)
	pasDeListeNulle(t, "home", acc)
	if len(acc.Calendar) != 365 || acc.Calendar[364].Day != "2026-03-31" {
		t.Fatalf("calendrier : %d jours, dernier %v", len(acc.Calendar), acc.Calendar[len(acc.Calendar)-1])
	}
	var total int64
	for _, j := range acc.Calendar {
		total += j.Commits
	}
	if total != 42 {
		t.Errorf("calendrier : %d commits, 42 attendus", total)
	}
	if acc.Month.Commits != 12 { // 6 commits in March per repository (see internal/sql/queries/README.md)
		t.Errorf("month: %d commits", acc.Month.Commits)
	}

	projets := lire[[]ProjectSummary](t, h, "/api/projects", 200)
	if len(projets) < 2 {
		t.Fatalf("projets : %+v", projets)
	}
	var atlas int64
	for _, p := range projets {
		if p.Name == "Atlas" {
			atlas = p.ID
		}
	}
	det := lire[ProjectDetail](t, h, "/api/projects/"+itoa(atlas), 200)
	pasDeListeNulle(t, "projet", det)
	if det.Project.Commits != 21 || len(det.Volume) != 52 || len(det.Languages) == 0 || len(det.Project.Technologies) == 0 {
		t.Errorf("projet Atlas : commits=%d semaines=%d languages=%d technos=%v", det.Project.Commits, len(det.Volume), len(det.Languages), det.Project.Technologies)
	}
	if det.Health.LastSHA == nil || det.Health.UpToDate {
		t.Errorf("Atlas health (fictional verified_commit a1b2c3d) : %+v", det.Health)
	}
	lire[APIError](t, h, "/api/projects/999999", 404)
	lire[APIError](t, h, "/api/projects/abc", 400)

	tech := lire[Technologies](t, h, "/api/technologies", 200)
	pasDeListeNulle(t, "technos", tech)
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
	ias := lire[[]Assistant](t, h, "/api/ai", 200)
	pasDeListeNulle(t, "ia", ias)

	g := lire[Graph](t, h, "/api/graph", 200)
	pasDeListeNulle(t, "graph", g)
	if len(g.Nodes) == 0 || len(g.Links) == 0 || len(g.Broken) != 3 {
		t.Errorf("graph: %d nodes, %d links, %d broken", len(g.Nodes), len(g.Links), len(g.Broken))
	}
	n := lire[NoteDetail](t, h, "/api/notes/"+itoa(g.Nodes[0].ID), 200)
	pasDeListeNulle(t, "note", n)
	ch := lire[GraphPath](t, h, "/api/graph/path?from="+itoa(g.Links[0].Source)+"&to="+itoa(g.Links[0].Target), 200)
	if len(ch.Path) != 2 {
		t.Errorf("chemin direct : %+v", ch)
	}
	lire[APIError](t, h, "/api/graph/path?from=x", 400)

	rech := lire[[]SearchHit](t, h, "/api/search?q=naive", 200)
	if len(rech) == 0 || !strings.Contains(rech[0].Title+rech[0].Excerpt, "<mark>") {
		t.Errorf("recherche : %+v", rech)
	}
	if v := lire[[]SearchHit](t, h, "/api/search?q=", 200); len(v) != 0 {
		t.Errorf("recherche vide : %+v", v)
	}
	lire[APIError](t, h, `/api/search?q=%22unclosed`, 400)
	lire[[]SearchHit](t, h, "/api/search?q=fixture&source=commit&limit=5", 200)
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// TestContractMirror checks that every interface in web/src/api/types.ts has exactly the JSON fields
// of the Go struct with the same name (contract.go).
func TestContratMiroir(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(racine(), "web", "src", "api", "types.ts"))
	if err != nil {
		t.Fatal(err)
	}
	blocs := regexp.MustCompile(`(?s)export interface (\w+)(?: extends [^{]+)? \{(.*?)\n\}`).FindAllStringSubmatch(string(src), -1)
	champTS := regexp.MustCompile(`(?m)^  (\w+)\??:`)
	types := map[string]reflect.Type{}
	for _, v := range []any{APIError{}, ActivityDay{}, Stats{}, ActiveProject{}, DormantProject{}, StaleCard{}, Home{},
		ProjectSummary{}, TimelineEvent{}, WeeklyVolume{}, LanguageShare{}, LanguagePoint{}, Hotspot{}, Health{}, AIActivity{},
		ProjectDetail{}, SankeyNode{}, SankeyLink{}, TechTrend{}, Technologies{}, Assistant{}, GraphNode{}, Graph{}, GraphPath{},
		NoteDetail{}, SearchHit{}, SQLRequest{}, SQLResult{}, SchemaTable{}, SavedQuery{}, UpdateEvent{}} {
		types[reflect.TypeOf(v).Name()] = reflect.TypeOf(v)
	}
	if len(blocs) < 25 {
		t.Fatalf("seulement %d interfaces lues dans types.ts", len(blocs))
	}
	for _, b := range blocs {
		typ, ok := types[b[1]]
		if !ok {
			t.Errorf("interface TS %s sans struct Go", b[1])
			continue
		}
		var ts []string
		for _, m := range champTS.FindAllStringSubmatch(b[2], -1) {
			ts = append(ts, m[1])
		}
		var goChamps []string
		for i := 0; i < typ.NumField(); i++ {
			if tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]; tag != "" {
				goChamps = append(goChamps, tag)
			}
		}
		sort.Strings(ts)
		sort.Strings(goChamps)
		if !reflect.DeepEqual(ts, goChamps) {
			t.Errorf("%s : TS %v ≠ Go %v", b[1], ts, goChamps)
		}
	}
}

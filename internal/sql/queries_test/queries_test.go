package queries_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/db"
	"github.com/Anarchiejs/Observatoire/internal/ingest"
	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
)

var update = flag.Bool("update", false, "rewrite reviewed JSON expectations and EXPLAIN plans")

const today = "2026-03-31"

func root() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(f), "../../.."))
}

func prepare(t *testing.T) *sql.DB {
	t.Helper()
	r := root()
	if _, err := os.Stat(filepath.Join(r, "testdata/repos/alpha/.git")); err != nil {
		t.Fatal("run go run ./scripts/fixtures/make-repos.go first:", err)
	}
	b, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	if err = db.Migrate(b); err != nil {
		t.Fatal(err)
	}
	summary, err := ingest.Import(context.Background(), b, ingest.Options{
		Vault: filepath.Join(r, "testdata/vault"),
		Repos: map[string]string{"Atlas": filepath.Join(r, "testdata/repos/alpha"), "Compass": filepath.Join(r, "testdata/repos/beta")},
		Today: today,
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Commits != 42 || len(summary.Warnings) > 0 {
		t.Fatalf("import: %+v", summary)
	}
	return b
}

func id(t *testing.T, b *sql.DB, table, column, value string) int64 {
	t.Helper()
	var n int64
	if err := b.QueryRow("SELECT id FROM "+table+" WHERE "+column+"=?", value).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Capture the exact generated SQL and bindings, rather than maintaining a second SQL copy.
type observation struct {
	*sql.DB
	text string
	args []any
}

func (o *observation) QueryContext(ctx context.Context, s string, args ...any) (*sql.Rows, error) {
	o.text = s
	o.args = args
	return o.DB.QueryContext(ctx, s, args...)
}
func (o *observation) QueryRowContext(ctx context.Context, s string, args ...any) *sql.Row {
	o.text = s
	o.args = args
	return o.DB.QueryRowContext(ctx, s, args...)
}

func plan(t *testing.T, o *observation) string {
	t.Helper()
	rows, err := o.DB.Query("EXPLAIN QUERY PLAN "+o.text, o.args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var a, b, c int
		var d string
		if err := rows.Scan(&a, &b, &c, &d); err != nil {
			t.Fatal(err)
		}
		details = append(details, d)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(details, "; ")
}

func TestExpectations(t *testing.T) {
	b := prepare(t)
	ctx := context.Background()
	atlas := id(t, b, "project", "name", "Atlas")
	note := id(t, b, "note", "title", "Atlas")
	concept := id(t, b, "note", "title", "Mapping")
	o := &observation{DB: b}
	q := gen.New(o)
	cases := []struct {
		name string
		arg  any
	}{
		{"Calendar", today}, {"PeriodStats", gen.PeriodStatsParams{StartDate: "2026-01-01", EndDate: today}},
		{"ActiveProjects", today}, {"DormantProjects", today}, {"StaleCards", nil},
		{"ListProjects", nil}, {"ProjectByID", atlas}, {"ProjectTechnologies", atlas}, {"Timeline", atlas},
		{"WeeklyVolume", gen.WeeklyVolumeParams{ProjectID: atlas, Today: today}},
		{"LatestLanguages", atlas}, {"LanguageEvolution", atlas}, {"Hotspots", atlas}, {"ProjectAI", &atlas}, {"ProjectNotes", atlas},
		{"SankeyLinks", nil}, {"TechnologyTrend", today}, {"UnassignedTechnologies", nil}, {"AIActivity", nil}, {"ProjectsByAI", nil}, {"AISessionsByMonth", today},
		{"GraphNodes", nil}, {"GraphLinks", nil}, {"BrokenLinks", nil}, {"NoteByID", note}, {"Neighbors", note},
		{"ShortestPath", gen.ShortestPathParams{StartID: note, EndID: concept}},
		{"Search", gen.SearchParams{Terms: "naive", RowLimit: 20}},
	}
	var plans []string
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := []reflect.Value{reflect.ValueOf(ctx)}
			if c.arg != nil {
				args = append(args, reflect.ValueOf(c.arg))
			}
			ret := reflect.ValueOf(q).MethodByName(c.name).Call(args)
			if !ret[1].IsNil() {
				t.Fatal(ret[1].Interface())
			}
			v := ret[0]
			if v.Kind() == reflect.Slice && v.IsNil() {
				v = reflect.MakeSlice(v.Type(), 0, 0)
			}
			data, err := json.MarshalIndent(v.Interface(), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, '\n')
			path := filepath.Join("expected", c.name+".json")
			if *update {
				if err := os.MkdirAll("expected", 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(data) {
				t.Fatalf("result differs from %s (review before -update)", path)
			}
			plans = append(plans, "- "+c.name+": "+plan(t, o))
		})
	}
	if *update && !t.Failed() {
		text := "# EXPLAIN QUERY PLAN\n\nCaptured on the migrated fixture database, without proposed indexes.\nFull scans for whole-graph/history aggregates are intentional; see INDEX_PROPOSED.md for selective scans.\n\n" + strings.Join(plans, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(root(), "internal/sql/queries/PLANS.md"), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInvariants(t *testing.T) {
	b := prepare(t)
	q := gen.New(b)
	ctx := context.Background()
	calendar, err := q.Calendar(ctx, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(calendar) != 365 || calendar[0].Day != "2025-04-01" || calendar[364].Day != today {
		t.Fatalf("calendar bounds: %+v", calendar)
	}
	var total, empty int64
	for _, day := range calendar {
		total += day.Commits
		if day.Commits == 0 {
			empty++
		}
	}
	// Each fixture has 20 changes plus one extra merge commit: 21 * 2.
	if total != 42 || empty == 0 {
		t.Fatalf("calendar commits=%d empty=%d", total, empty)
	}
	stats, err := q.PeriodStats(ctx, gen.PeriodStatsParams{StartDate: "2026-01-01", EndDate: today})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Commits != 42 || stats.ActiveProjects != 2 || stats.Sessions != 0 {
		t.Fatalf("period: %+v", stats)
	}
	// Changes 15..20 fall on March 5, 8, 11, 14, 17, 20 in each repository: 12.
	active, err := q.ActiveProjects(ctx, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 || active[0].Commits30d != 6 || active[1].Commits30d != 6 {
		t.Fatalf("active: %+v", active)
	}
	a := id(t, b, "project", "name", "Atlas")
	// Fixtures use active/archived, so add test-only rows for the exact "in progress" status.
	if _, err := b.Exec("UPDATE project SET status='in progress' WHERE id=?", a); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Exec("INSERT INTO project(name,status) VALUES ('Never','in progress')"); err != nil {
		t.Fatal(err)
	}
	dormant, err := q.DormantProjects(ctx, "2026-05-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(dormant) != 2 || dormant[0].Name != "Atlas" || dormant[1].Days != -1 {
		t.Fatalf("dormant: %+v", dormant)
	}
	start := id(t, b, "note", "title", "Atlas")
	end := id(t, b, "note", "title", "Mapping")
	orphan := id(t, b, "note", "title", "Naïve")
	for _, pair := range [][2]int64{{start, end}, {end, start}, {start, start}, {start, orphan}} {
		path, err := q.ShortestPath(ctx, gen.ShortestPathParams{StartID: pair[0], EndID: pair[1]})
		if err != nil {
			t.Fatal(err)
		}
		want := 2
		if pair[0] == pair[1] {
			want = 1
		}
		if pair[1] == orphan {
			want = 0
		}
		if len(path) != want || (want > 0 && (path[0].ID != pair[0] || path[want-1].ID != pair[1])) {
			t.Fatalf("path %v: %+v", pair, path)
		}
	}
	hits, err := q.Search(ctx, gen.SearchParams{Terms: "naive", RowLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].NoteID == nil || *hits[0].NoteID != orphan {
		t.Fatalf("accent search: %+v", hits)
	}
	volume, err := q.WeeklyVolume(ctx, gen.WeeklyVolumeParams{ProjectID: a, Today: today})
	if err != nil {
		t.Fatal(err)
	}
	if len(volume) != 52 || volume[51].Week != "2026-03-30" {
		t.Fatalf("weeks: %+v", volume)
	}
}

func TestSQLASCII(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(root(), "internal/sql/queries/*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, c := range content {
			if c > 127 {
				t.Error(fmt.Sprintf("%s: non-ASCII at byte %d", f, i))
			}
		}
	}
}

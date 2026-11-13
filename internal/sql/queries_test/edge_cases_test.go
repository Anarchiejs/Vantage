package queries_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Anarchiejs/Vantage/internal/sql/gen"
)

func TestSearchFilters(t *testing.T) {
	b := prepare(t)
	q := gen.New(b)
	ctx := context.Background()
	cases := []struct {
		name  string
		p     gen.SearchParams
		empty bool
	}{
		{"commit", gen.SearchParams{Terms: "fixture", Source: "commit", Project: "Atlas", Since: "2026-01-01", Until: "2026-03-31", RowLimit: 3}, false},
		{"note", gen.SearchParams{Terms: "Atlas", Source: "note", Project: "Atlas", Folder: "Projects", RowLimit: 20}, false},
		{"date", gen.SearchParams{Terms: "Atlas", Source: "note", Since: "2030-01-01", RowLimit: 20}, true},
		{"folder", gen.SearchParams{Terms: "fixture", Source: "commit", Folder: "Projects", RowLimit: 20}, true},
		{"project", gen.SearchParams{Terms: "naive", Project: "Atlas", RowLimit: 20}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, err := q.Search(ctx, c.p)
			if err != nil {
				t.Fatal(err)
			}
			if (len(rows) == 0) != c.empty {
				t.Fatalf("unexpected rows: %+v", rows)
			}
			if int64(len(rows)) > c.p.RowLimit {
				t.Fatal("limit ignored")
			}
			for _, r := range rows {
				if c.p.Source != nil && r.Source != c.p.Source {
					t.Fatalf("source: %+v", r)
				}
				if c.p.Project != nil && (r.Project == nil || *r.Project != c.p.Project) {
					t.Fatalf("project: %+v", r)
				}
				if !strings.Contains(r.Title, "\uE000") && !strings.Contains(r.Excerpt, "\uE000") {
					t.Fatalf("highlight: %+v", r)
				}
			}
		})
	}
}

func TestPathDepthAndCycle(t *testing.T) {
	b := prepare(t)
	ctx := context.Background()
	q := gen.New(b)
	var ids []int64
	for i := 0; i < 8; i++ {
		res, err := b.Exec("INSERT INTO note(path,folder,title,content_hash) VALUES (?,'Concepts',?,'test')", fmt.Sprintf("chain/%d.md", i), fmt.Sprintf("chain-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		n, err := res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, n)
		if i > 0 {
			if _, err := b.Exec("INSERT INTO link(source_id,target_title,target_id) VALUES (?,?,?)", ids[i-1], fmt.Sprintf("chain-%d", i), n); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A backward edge creates a cycle and a duplicate undirected connection.
	if _, err := b.Exec("INSERT INTO link(source_id,target_title,target_id) VALUES (?,'cycle',?)", ids[1], ids[0]); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		target int
		size   int
	}{{6, 7}, {7, 0}} {
		rows, err := q.ShortestPath(ctx, gen.ShortestPathParams{StartID: ids[0], EndID: ids[c.target]})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != c.size {
			t.Fatalf("target %d: %+v", c.target, rows)
		}
		for i, r := range rows {
			if r.ID != ids[i] {
				t.Fatalf("path order: %+v", rows)
			}
		}
	}
}

func TestKnownCommitPrefix(t *testing.T) {
	b := prepare(t)
	q := gen.New(b)
	ctx := context.Background()
	a := id(t, b, "project", "name", "Atlas")
	var sha string
	if err := b.QueryRow("SELECT c.sha FROM repository d JOIN commit_git c ON c.repository_id=d.id WHERE d.project_id=? ORDER BY c.date_utc,c.sha LIMIT 1", a).Scan(&sha); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Exec("UPDATE project SET verified_commit=? WHERE id=?", sha[:10], a); err != nil {
		t.Fatal(err)
	}
	rows, err := q.StaleCards(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Name != "Atlas" || rows[0].CommitsSince != 20 || rows[1].CommitsSince != -1 {
		t.Fatalf("stale: %+v", rows)
	}
	if _, err := b.Exec("UPDATE project SET verified_commit=(SELECT substr(last_imported_sha,1,10) FROM repository WHERE project_id=?) WHERE id=?", a, a); err != nil {
		t.Fatal(err)
	}
	rows, err = q.StaleCards(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "Compass" {
		t.Fatalf("current prefix: %+v", rows)
	}
}

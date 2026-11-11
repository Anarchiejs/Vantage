package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Anarchiejs/Observatoire/internal/db"
)

func testServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	rw, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rw.Close() })
	if err := db.Migrate(rw); err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Exec(`INSERT INTO project (name) VALUES ('Alpha'), ('Beta')`); err != nil {
		t.Fatal(err)
	}
	ro, err := db.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ro.Close() })
	s, err := New(Config{
		Address: "127.0.0.1:0", DB: rw, ReadDB: ro,
		Queries: filepath.Join(dir, "queries"),
		UI:      fstest.MapFS{"index.html": {Data: []byte("<html>home</html>")}, "assets/a.js": {Data: []byte("x")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, s.Handler()
}

func callHandler(h http.Handler, method, url, host, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, strings.NewReader(body))
	r.Host = host
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestLoopbackOnly(t *testing.T) {
	if _, err := New(Config{Address: "0.0.0.0:7878"}); err == nil {
		t.Fatal("listening on 0.0.0.0 was accepted")
	}
	if _, err := New(Config{Address: "192.168.1.10:7878"}); err == nil {
		t.Fatal("listening on the local network was accepted")
	}
}

func TestHostGuard(t *testing.T) {
	_, h := testServer(t)
	if w := callHandler(h, "GET", "/api/schema", "evil.example:7878", ""); w.Code != http.StatusForbidden {
		t.Fatalf("foreign Host : %d", w.Code)
	}
	if w := callHandler(h, "GET", "/api/schema", "127.0.0.1:7878", ""); w.Code != http.StatusOK {
		t.Fatalf("Host local : %d %s", w.Code, w.Body)
	}
	// A POST without JSON (a form from another site) is refused.
	r := httptest.NewRequest("POST", "/api/sql", strings.NewReader("sql=select+1"))
	r.Host = "127.0.0.1:7878"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("POST formulaire : %d", w.Code)
	}
	// A foreign Origin is refused.
	r = httptest.NewRequest("POST", "/api/sql", strings.NewReader(`{"sql":"select 1"}`))
	r.Host = "127.0.0.1:7878"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign Origin : %d", w.Code)
	}
}

func TestConsoleReadOnly(t *testing.T) {
	_, h := testServer(t)
	w := callHandler(h, "POST", "/api/sql", "127.0.0.1", `{"sql":"SELECT name FROM project ORDER BY name"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var res SQLResult
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Rows) != 2 || res.Rows[0][0] != "Alpha" || res.Columns[0] != "name" {
		t.Fatalf("result: %+v", res)
	}

	for _, sql := range []string{
		`DELETE FROM project`,
		`INSERT INTO project (name) VALUES ('x')`,
		`DROP TABLE project`,
		`SELECT 1; DELETE FROM project`,
		`WITH x AS (SELECT 1) DELETE FROM project`,
		`ATTACH DATABASE 'autre.db' AS a`,
		`PRAGMA query_only = 0`,
		`PRAGMA query_only(0)`,
	} {
		body, _ := json.Marshal(SQLRequest{SQL: sql})
		if w := callHandler(h, "POST", "/api/sql", "127.0.0.1", string(body)); w.Code == http.StatusOK {
			t.Errorf("accepted: %s", sql)
		}
	}
	w = callHandler(h, "POST", "/api/sql", "127.0.0.1", `{"sql":"SELECT count(*) FROM project"}`)
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Rows[0][0] != float64(2) {
		t.Fatalf("the database changed : %v", res.Rows)
	}
}

func TestConsoleTruncates(t *testing.T) {
	_, h := testServer(t)
	w := callHandler(h, "POST", "/api/sql", "127.0.0.1",
		`{"sql":"WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 5000) SELECT i FROM n"}`)
	var res SQLResult
	json.Unmarshal(w.Body.Bytes(), &res)
	if !res.Truncated || len(res.Rows) != maxConsoleRows {
		t.Fatalf("truncated=%v rows=%d", res.Truncated, len(res.Rows))
	}
}

func TestSavedQueries(t *testing.T) {
	s, h := testServer(t)
	for _, name := range []string{"../evil", "a/b", `a\b`, "Notlower", "", strings.Repeat("a", 65)} {
		body, _ := json.Marshal(SavedQuery{Name: name, SQL: "select 1"})
		if w := callHandler(h, "POST", "/api/queries", "127.0.0.1", string(body)); w.Code != http.StatusBadRequest {
			t.Errorf("name %q accepted (%d)", name, w.Code)
		}
	}
	if w := callHandler(h, "POST", "/api/queries", "127.0.0.1", `{"name":"top-projects","sql":"select 1"}`); w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(s.cfg.Queries, "top-projects.sql")); err != nil {
		t.Fatal(err)
	}
	w := callHandler(h, "GET", "/api/queries", "127.0.0.1", "")
	var saved []SavedQuery
	json.Unmarshal(w.Body.Bytes(), &saved)
	if len(saved) != 1 || saved[0].Name != "top-projects" {
		t.Fatalf("%+v", saved)
	}
}

func TestInterfaceSPA(t *testing.T) {
	_, h := testServer(t)
	if w := callHandler(h, "GET", "/projects/3", "127.0.0.1", ""); !strings.Contains(w.Body.String(), "home") {
		t.Fatalf("client route: %d %s", w.Code, w.Body)
	}
	if w := callHandler(h, "GET", "/api/unknown", "127.0.0.1", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown API: %d", w.Code)
	}
}

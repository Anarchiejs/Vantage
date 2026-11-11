package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxConsoleRows    = 1000
	maxConsoleBytes   = 16 << 20 // result budget before JSON encoding
	concurrentQueries = 2
)

// consoleTimeout bounds each console query (a variable so tests can shorten it).
var consoleTimeout = 5 * time.Second

// executeSQL runs a console query on a read-only connection protected by
// allowConsole (sql_guard.go): mode=ro blocks writes to the file, the authorizer refuses
// ATTACH, write PRAGMAs, and transactions, and the deadline also covers scanning rows.
func (s *Server) executeSQL(w http.ResponseWriter, r *http.Request) {
	var request SQLRequest
	if !readJSON(w, r, &request) {
		return
	}
	query := strings.TrimSpace(request.SQL)
	if query == "" {
		writeError(w, http.StatusBadRequest, "empty query")
		return
	}
	select {
	case s.console <- struct{}{}:
		defer func() { <-s.console }()
	default:
		writeError(w, http.StatusTooManyRequests, "another console query is already running")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), consoleTimeout)
	defer cancel()
	conn, err := s.cfg.ReadDB.Conn(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, sqlMessage(ctx, err))
		return
	}
	defer conn.Close()
	stop, err := protectConnection(ctx, conn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer stop()

	started := time.Now()
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		writeError(w, sqlStatus(err), sqlMessage(ctx, err))
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res := SQLResult{Columns: columns, Rows: [][]any{}}
	size := 0
	for rows.Next() {
		if len(res.Rows) == maxConsoleRows || size > maxConsoleBytes {
			res.Truncated = true
			break
		}
		vals := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		for i, v := range vals {
			size += 8
			if b, ok := v.([]byte); ok {
				if utf8.Valid(b) {
					vals[i] = string(b)
					// A control character becomes "\u0000" in JSON: six bytes.
					size += 6 * len(b)
				} else {
					vals[i] = fmt.Sprintf("<blob %d bytes>", len(b))
				}
			} else if t, ok := v.(string); ok {
				size += 6 * len(t)
			}
		}
		res.Rows = append(res.Rows, vals)
	}
	if err := rows.Err(); err != nil {
		writeError(w, sqlStatus(err), sqlMessage(ctx, err))
		return
	}
	res.Duration = float64(time.Since(started).Microseconds()) / 1000
	writeJSON(w, res)
}

func sqlStatus(err error) int {
	if strings.Contains(err.Error(), "not authorized") {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}

func sqlMessage(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Sprintf("query stopped after %s", consoleTimeout)
	}
	if strings.Contains(err.Error(), "not authorized") {
		return "command not allowed in the console: read-only (no ATTACH, write PRAGMA, or transaction)"
	}
	return err.Error()
}

func (s *Server) schema(w http.ResponseWriter, r *http.Request) {
	tables, err := s.cfg.ReadDB.QueryContext(r.Context(),
		`SELECT name FROM sqlite_schema
		 WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'goose_%'
		   AND name NOT LIKE 'search_index_%'
		 ORDER BY name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var names []string
	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			tables.Close()
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		names = append(names, name)
	}
	tables.Close()

	res := []SchemaTable{}
	for _, name := range names {
		table := SchemaTable{Table: name, Columns: []SchemaColumn{}}
		cols, err := s.cfg.ReadDB.QueryContext(r.Context(), `SELECT name, type FROM pragma_table_xinfo(?)`, name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for cols.Next() {
			var column SchemaColumn
			if err := cols.Scan(&column.Name, &column.Type); err != nil {
				cols.Close()
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			table.Columns = append(table.Columns, column)
		}
		cols.Close()
		res = append(res, table)
	}
	writeJSON(w, res)
}

var queryName = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

func (s *Server) listQueries(w http.ResponseWriter, r *http.Request) {
	res := []SavedQuery{}
	entries, err := os.ReadDir(s.cfg.Queries)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".sql")
		if entry.IsDir() || !ok || !queryName.MatchString(name) {
			continue
		}
		body, err := os.ReadFile(filepath.Join(s.cfg.Queries, entry.Name()))
		if err != nil {
			continue
		}
		res = append(res, SavedQuery{Name: name, SQL: string(body)})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	writeJSON(w, res)
}

func (s *Server) saveQuery(w http.ResponseWriter, r *http.Request) {
	var query SavedQuery
	if !readJSON(w, r, &query) {
		return
	}
	// The name is the only path segment: the regex forbids "..", "/", and "\".
	if !queryName.MatchString(query.Name) {
		writeError(w, http.StatusBadRequest, "invalid name: lowercase letters, digits, and hyphens only")
		return
	}
	if strings.TrimSpace(query.SQL) == "" {
		writeError(w, http.StatusBadRequest, "empty query")
		return
	}
	if err := os.MkdirAll(s.cfg.Queries, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(filepath.Join(s.cfg.Queries, query.Name+".sql"), []byte(query.SQL), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, query)
}

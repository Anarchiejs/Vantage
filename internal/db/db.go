// Package db opens the SQLite database and applies migrations.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/Anarchiejs/Observatoire/migrations"
)

// Open opens the database read-write with the project's PRAGMAs.
func Open(path string) (*sql.DB, error) {
	return open(path, false)
}

// OpenReadOnly opens the database with mode=ro: every write fails (SQL console).
func OpenReadOnly(path string) (*sql.DB, error) {
	return open(path, true)
}

func open(path string, readOnly bool) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	if readOnly {
		q.Set("mode", "ro")
		q.Add("_pragma", "query_only(1)")
	} else {
		q.Add("_pragma", "journal_mode(WAL)")
		q.Add("_pragma", "synchronous(NORMAL)")
	}
	dsn := "file:" + filepath.ToSlash(abs) + "?" + q.Encode()
	base, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := base.Ping(); err != nil {
		base.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return base, nil
}

// ErrLegacySchema means the file was created before the English schema rename.
// Goose will not rebuild tables that version 1 already applied, so the database
// has to be deleted and imported again.
var ErrLegacySchema = errors.New("this database uses the retired French schema (table projet): delete the database file and its -wal and -shm companions, then import the vault again (see CHANGELOG.md)")

// Migrate brings the schema up to the latest version.
func Migrate(base *sql.DB) error {
	if err := refuseLegacySchema(base); err != nil {
		return err
	}
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(base, ".")
}

func refuseLegacySchema(base *sql.DB) error {
	var name string
	err := base.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'projet'`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrLegacySchema
}

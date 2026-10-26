// Package db ouvre la base SQLite et applique les migrations.
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/Anarchiejs/Observatoire/migrations"
)

// Ouvrir ouvre la base en lecture-écriture avec les PRAGMA du projet.
func Ouvrir(chemin string) (*sql.DB, error) {
	return ouvrir(chemin, false)
}

// OuvrirLectureSeule ouvre la base en mode=ro : toute écriture échoue (console SQL).
func OuvrirLectureSeule(chemin string) (*sql.DB, error) {
	return ouvrir(chemin, true)
}

func ouvrir(chemin string, lectureSeule bool) (*sql.DB, error) {
	abs, err := filepath.Abs(chemin)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	if lectureSeule {
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
		return nil, fmt.Errorf("ouverture de %s : %w", chemin, err)
	}
	return base, nil
}

// Migrer amène le schéma à la dernière version.
func Migrer(base *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(base, ".")
}

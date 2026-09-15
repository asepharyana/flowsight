// Package store persists snapshots, derived tables, routines, alerts, reports,
// the accuracy ledger, and the credit ledger on SQLite (modernc.org/sqlite,
// pure Go — no CGO so demo machines without gcc still build).
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps database/sql with FlowSight helpers.
type DB struct {
	*sql.DB
	Path string
}

// Open connects to path (created if missing) and applies numbered migrations.
func Open(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	sqldb.SetMaxOpenConns(1)
	db := &DB{DB: sqldb, Path: path}
	if err := db.migrate(); err != nil {
		sqldb.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) migrate() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, "migrations/"+e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		b, err := migrationsFS.ReadFile(n)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(b)); err != nil {
			return fmt.Errorf("store: migration %s: %w", n, err)
		}
	}
	return nil
}

// Meta helpers persist incremental cursors (quarterly_since, news_since,
// filings_since) and dedup markers so restarts resume incrementally.

// GetMeta returns a meta value or "".
func (db *DB) GetMeta(key string) string {
	var v string
	_ = db.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v)
	return v
}

// SetMeta upserts a meta value.
func (db *DB) SetMeta(key, value string) error {
	_, err := db.Exec(`INSERT INTO meta(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

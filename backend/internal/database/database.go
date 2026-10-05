// Package database opens the SQLite database the server uses.
package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite file at path. Writers wait up to five seconds for the
// lock instead of failing with SQLITE_BUSY, and write-ahead logging lets
// readers run while a write is in progress.
func Open(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

package driver

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func ProbeFTS5() error {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return fmt.Errorf("open sqlite probe: %w", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE VIRTUAL TABLE mem_probe USING fts5(body)`); err != nil {
		return fmt.Errorf("probe sqlite fts5: %w", err)
	}
	return nil
}

func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("missing sqlite path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}
	return db, nil
}

func OpenReadOnly(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("missing sqlite path")
	}
	dsn := (&url.URL{
		Scheme: "file",
		Path:   path,
		RawQuery: url.Values{
			"mode": {"ro"},
		}.Encode(),
	}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite read-only: %w", err)
	}
	if _, err := db.Exec(`PRAGMA query_only = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure sqlite read-only: %w", err)
	}
	return db, nil
}

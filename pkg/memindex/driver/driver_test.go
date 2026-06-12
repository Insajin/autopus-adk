package driver

import (
	"path/filepath"
	"testing"
)

// TestProbeFTS5 verifies the FTS5 capability probe succeeds on the bundled
// modernc.org/sqlite build (FTS5 is compiled in).
func TestProbeFTS5(t *testing.T) {
	if err := ProbeFTS5(); err != nil {
		t.Fatalf("ProbeFTS5 should succeed with FTS5-enabled sqlite: %v", err)
	}
}

// TestOpenEmptyPath asserts Open rejects an empty path before touching disk.
func TestOpenEmptyPath(t *testing.T) {
	db, err := Open("")
	if err == nil {
		t.Fatal("Open(\"\") should return an error")
	}
	if db != nil {
		t.Fatal("Open(\"\") should return a nil handle on error")
	}
}

// TestOpenCreatesDirAndUsableDB asserts Open creates the parent directory,
// opens a writable database, and applies the busy_timeout pragma.
func TestOpenCreatesDirAndUsableDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "sub", "index.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() { _ = db.Close() }()

	// The database must be writable: create a table and insert a row.
	if _, err := db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table on opened db: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO t (v) VALUES ('hello')`); err != nil {
		t.Fatalf("insert on opened db: %v", err)
	}

	var got string
	if err := db.QueryRow(`SELECT v FROM t WHERE id = 1`).Scan(&got); err != nil {
		t.Fatalf("select on opened db: %v", err)
	}
	if got != "hello" {
		t.Fatalf("round-trip value = %q, want %q", got, "hello")
	}

	// busy_timeout pragma must be applied (5000ms as configured by Open).
	var timeout int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		t.Fatalf("read busy_timeout pragma: %v", err)
	}
	if timeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", timeout)
	}
}

// TestOpenReadOnlyEmptyPath asserts OpenReadOnly rejects an empty path.
func TestOpenReadOnlyEmptyPath(t *testing.T) {
	db, err := OpenReadOnly("")
	if err == nil {
		t.Fatal("OpenReadOnly(\"\") should return an error")
	}
	if db != nil {
		t.Fatal("OpenReadOnly(\"\") should return a nil handle on error")
	}
}

// TestOpenReadOnlyRejectsWrites asserts that a read-only handle can read an
// existing database but rejects mutations via the query_only pragma.
func TestOpenReadOnlyRejectsWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.db")

	// Seed the database with a writable handle first.
	wdb, err := Open(path)
	if err != nil {
		t.Fatalf("seed Open: %v", err)
	}
	if _, err := wdb.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	if _, err := wdb.Exec(`INSERT INTO t (v) VALUES ('seed')`); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	_ = wdb.Close()

	rdb, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly returned error: %v", err)
	}
	defer func() { _ = rdb.Close() }()

	// Read must succeed and observe the seeded row.
	var got string
	if err := rdb.QueryRow(`SELECT v FROM t WHERE id = 1`).Scan(&got); err != nil {
		t.Fatalf("read-only select: %v", err)
	}
	if got != "seed" {
		t.Fatalf("read-only value = %q, want %q", got, "seed")
	}

	// query_only pragma must be ON.
	var queryOnly int
	if err := rdb.QueryRow(`PRAGMA query_only`).Scan(&queryOnly); err != nil {
		t.Fatalf("read query_only pragma: %v", err)
	}
	if queryOnly != 1 {
		t.Fatalf("query_only = %d, want 1", queryOnly)
	}

	// A write must be rejected by the read-only connection.
	if _, err := rdb.Exec(`INSERT INTO t (v) VALUES ('nope')`); err == nil {
		t.Fatal("write through read-only handle should fail")
	}
}

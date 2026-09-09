//go:build darwin || linux

package ompprobe

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// sentinelSecret stands in for bytes the exporter must never republish: a
// root-owned file the canary UID could try to trick the runner into copying.
const sentinelSecret = "PROTECTED-SENTINEL-BYTES-a1b2c3d4"

type exportFixture struct {
	options     ExportOptions
	base        string
	sourcePath  string
	destination string
	sentinel    string
}

func newExportFixture(t *testing.T) exportFixture {
	t.Helper()
	base := realTempDir(t)
	sourceRoot := filepath.Join(base, "workspace")
	makeFixtureDirectory(t, sourceRoot)
	makeFixtureDirectory(t, filepath.Join(sourceRoot, "probe"))
	destinationRoot := filepath.Join(base, "retained")
	makeFixtureDirectory(t, destinationRoot)
	sourcePath := filepath.Join(sourceRoot, "probe", RecordFileName)
	writeFixtureFile(t, sourcePath, fixtureBody(t))
	sentinel := filepath.Join(base, "sentinel")
	writeFixtureFile(t, sentinel, []byte(sentinelSecret))
	options := ExportOptions{
		SourceRoot:      sourceRoot,
		SourceRelative:  filepath.Join("probe", RecordFileName),
		DestinationRoot: destinationRoot,
		DestinationName: "probe-0f1e2d3c4b5a69788796a5b4c3d2e1f0.jsonl",
		SourceUID:       uint32(os.Getuid()),
		OwnerUID:        uint32(os.Getuid()),
		OwnerGID:        uint32(os.Getgid()),
	}
	return exportFixture{
		options:     options,
		base:        base,
		sourcePath:  sourcePath,
		destination: filepath.Join(destinationRoot, options.DestinationName),
		sentinel:    sentinel,
	}
}

// assertNothingPublished proves the retained directory is untouched and that no
// protected byte reached it.
func assertNothingPublished(t *testing.T, fixture exportFixture) {
	t.Helper()
	if _, err := os.Lstat(fixture.destination); !os.IsNotExist(err) {
		t.Fatalf("destination exists after a refused export: %v", err)
	}
	entries, err := os.ReadDir(fixture.options.DestinationRoot)
	if err != nil {
		t.Fatalf("read retained directory: %v", err)
	}
	for _, entry := range entries {
		body, readErr := os.ReadFile(filepath.Join(fixture.options.DestinationRoot, entry.Name()))
		if readErr == nil && bytes.Contains(body, []byte(sentinelSecret)) {
			t.Fatalf("protected bytes were republished as %s", entry.Name())
		}
	}
	if len(entries) != 0 {
		t.Fatalf("retained directory holds %d unexpected entries", len(entries))
	}
}

func TestExport_PublishesRevalidatedRecordsIntoANewPrivateFile(t *testing.T) {
	fixture := newExportFixture(t)
	summary, err := Export(fixture.options)
	if err != nil {
		t.Fatalf("export valid probe records: %v", err)
	}
	if summary.Accepted != 2 || summary.Rejected != 0 {
		t.Fatalf("summary is accepted=%d rejected=%d, want 2/0", summary.Accepted, summary.Rejected)
	}
	published, err := os.ReadFile(fixture.destination)
	if err != nil {
		t.Fatalf("read retained file: %v", err)
	}
	if !bytes.Equal(published, fixtureBody(t)) {
		t.Fatal("retained bytes are not the re-serialized validated records")
	}
	var stat unix.Stat_t
	if err := unix.Lstat(fixture.destination, &stat); err != nil {
		t.Fatalf("stat retained file: %v", err)
	}
	if uint32(stat.Mode)&unix.S_IFMT != unix.S_IFREG || uint32(stat.Mode)&0o7777 != 0o600 {
		t.Fatalf("retained mode is %o, want a regular 0600 file", stat.Mode)
	}
	if stat.Uid != fixture.options.OwnerUID || stat.Gid != fixture.options.OwnerGID {
		t.Fatalf("retained owner is %d:%d, want %d:%d",
			stat.Uid, stat.Gid, fixture.options.OwnerUID, fixture.options.OwnerGID)
	}
	if uint64(stat.Nlink) != 1 || stat.Size != int64(len(published)) {
		t.Fatalf("retained file has %d links and %d bytes", stat.Nlink, stat.Size)
	}
}

func TestExport_RefusesSymlinkedSourceComponents(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(*testing.T, *exportFixture)
	}{
		{
			name: "record file is a symlink to a protected file",
			arrange: func(t *testing.T, fixture *exportFixture) {
				removeFixturePath(t, fixture.sourcePath)
				linkFixtureSymlink(t, fixture.sentinel, fixture.sourcePath)
			},
		},
		{
			name: "record directory is a symlink to a valid directory",
			arrange: func(t *testing.T, fixture *exportFixture) {
				other := filepath.Join(fixture.base, "other")
				makeFixtureDirectory(t, other)
				writeFixtureFile(t, filepath.Join(other, RecordFileName), fixtureBody(t))
				removeFixturePath(t, fixture.sourcePath)
				removeFixturePath(t, filepath.Dir(fixture.sourcePath))
				linkFixtureSymlink(t, other, filepath.Dir(fixture.sourcePath))
			},
		},
		{
			name: "source root is reached through a symlink",
			arrange: func(t *testing.T, fixture *exportFixture) {
				alias := filepath.Join(fixture.base, "alias")
				linkFixtureSymlink(t, fixture.options.SourceRoot, alias)
				fixture.options.SourceRoot = alias
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExportFixture(t)
			test.arrange(t, &fixture)
			if _, err := Export(fixture.options); err == nil {
				t.Fatal("export followed a symlinked component")
			}
			assertNothingPublished(t, fixture)
		})
	}
}

func TestExport_RefusesSwappedSourceObjectsWithoutLeakingBytes(t *testing.T) {
	tests := []struct {
		name string
		swap func(*testing.T, exportFixture)
	}{
		{
			name: "hardlink to a protected file",
			swap: func(t *testing.T, fixture exportFixture) {
				removeFixturePath(t, fixture.sourcePath)
				if err := os.Link(fixture.sentinel, fixture.sourcePath); err != nil {
					t.Fatalf("create hardlink: %v", err)
				}
			},
		},
		{
			name: "named pipe in place of the record file",
			swap: func(t *testing.T, fixture exportFixture) {
				removeFixturePath(t, fixture.sourcePath)
				if err := unix.Mkfifo(fixture.sourcePath, 0o600); err != nil {
					t.Fatalf("create fifo: %v", err)
				}
			},
		},
		{
			name: "directory in place of the record file",
			swap: func(t *testing.T, fixture exportFixture) {
				removeFixturePath(t, fixture.sourcePath)
				makeFixtureDirectory(t, fixture.sourcePath)
			},
		},
		{
			name: "protected content copied over the record file",
			swap: func(t *testing.T, fixture exportFixture) {
				writeFixtureFile(t, fixture.sourcePath, []byte(sentinelSecret+"\n"))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExportFixture(t)
			test.swap(t, fixture)
			if _, err := Export(fixture.options); err == nil {
				t.Fatal("export accepted a swapped source object")
			}
			assertNothingPublished(t, fixture)
		})
	}
}

func TestExport_RefusesASourceOwnedByAnotherIdentity(t *testing.T) {
	fixture := newExportFixture(t)
	fixture.options.SourceUID++
	if _, err := Export(fixture.options); err != errExportSource {
		t.Fatalf("foreign-owned source error is %v, want %v", err, errExportSource)
	}
	assertNothingPublished(t, fixture)
}

func removeFixturePath(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove fixture path: %v", err)
	}
}

func linkFixtureSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
}

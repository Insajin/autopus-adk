//go:build darwin || linux

package ompprobe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExport_NeverOverwritesAnExistingDestination(t *testing.T) {
	fixture := newExportFixture(t)
	writeFixtureFile(t, fixture.destination, []byte(sentinelSecret))
	if _, err := Export(fixture.options); err != errExportPublish {
		t.Fatalf("existing destination error is %v, want %v", err, errExportPublish)
	}
	body, err := os.ReadFile(fixture.destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(body) != sentinelSecret {
		t.Fatal("an existing destination was truncated or replaced")
	}
}

func TestExport_RefusesAnUntrustedDestinationDirectory(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
	}{
		{name: "world writable", mode: 0o777},
		{name: "group writable", mode: 0o770},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExportFixture(t)
			if err := os.Chmod(fixture.options.DestinationRoot, test.mode); err != nil {
				t.Fatalf("chmod destination root: %v", err)
			}
			if _, err := Export(fixture.options); err != errExportDestination {
				t.Fatalf("error is %v, want %v", err, errExportDestination)
			}
			if _, err := os.Lstat(fixture.destination); !os.IsNotExist(err) {
				t.Fatal("records were published into a writable-by-others directory")
			}
		})
	}
}

func TestExport_RejectsUnsafeArguments(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ExportOptions, string)
	}{
		{
			name:   "relative source root",
			mutate: func(options *ExportOptions, _ string) { options.SourceRoot = "workspace" },
		},
		{
			name: "unclean source root",
			mutate: func(options *ExportOptions, base string) {
				options.SourceRoot = base + "/../" + filepath.Base(base) + "/workspace"
			},
		},
		{
			name:   "filesystem root as destination",
			mutate: func(options *ExportOptions, _ string) { options.DestinationRoot = "/" },
		},
		{
			name:   "absolute source relative path",
			mutate: func(options *ExportOptions, _ string) { options.SourceRelative = "/etc/passwd" },
		},
		{
			name:   "escaping source relative path",
			mutate: func(options *ExportOptions, _ string) { options.SourceRelative = "../sentinel" },
		},
		{
			name:   "empty source relative path",
			mutate: func(options *ExportOptions, _ string) { options.SourceRelative = "" },
		},
		{
			name:   "destination name without the probe prefix",
			mutate: func(options *ExportOptions, _ string) { options.DestinationName = "records.jsonl" },
		},
		{
			name:   "destination name without the jsonl suffix",
			mutate: func(options *ExportOptions, _ string) { options.DestinationName = "probe-abc.json" },
		},
		{
			name:   "destination name with a path separator",
			mutate: func(options *ExportOptions, _ string) { options.DestinationName = "probe-a/b.jsonl" },
		},
		{
			name:   "destination name with an empty nonce",
			mutate: func(options *ExportOptions, _ string) { options.DestinationName = "probe-.jsonl" },
		},
		{
			name:   "destination name with a traversal nonce",
			mutate: func(options *ExportOptions, _ string) { options.DestinationName = "probe-..jsonl" },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExportFixture(t)
			options := fixture.options
			test.mutate(&options, fixture.base)
			if _, err := Export(options); err != errExportOptions {
				t.Fatalf("error is %v, want %v", err, errExportOptions)
			}
			assertNothingPublished(t, fixture)
		})
	}
}

func TestExport_ValidatesEveryRecordBeforeTouchingTheDestination(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{name: "smuggled body field", body: spliceRecordField(exportFixtureLine(), `"prompt":"secret"`)},
		{name: "not json", body: []byte(sentinelSecret + "\n")},
		{name: "no trailing newline", body: exportFixtureLine()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExportFixture(t)
			writeFixtureFile(t, fixture.sourcePath, test.body)
			summary, err := Export(fixture.options)
			if err == nil {
				t.Fatal("an unvalidated record file was accepted")
			}
			if summary.Accepted != 0 {
				t.Fatalf("summary accepted %d records from a refused file", summary.Accepted)
			}
			assertNothingPublished(t, fixture)
		})
	}
}

func TestExport_ReportsRejectedRecordCountsWithoutPublishing(t *testing.T) {
	fixture := newExportFixture(t)
	valid := fixtureBody(t)
	invalid := spliceRecordField(exportFixtureLine(), `"credential":"sk-live-000"`)
	writeFixtureFile(t, fixture.sourcePath, append(append([]byte(nil), valid...), invalid...))
	summary, err := Export(fixture.options)
	if err != errRecordUnknownField {
		t.Fatalf("error is %v, want an unknown-field rejection", err)
	}
	if summary.Accepted != 2 || summary.Rejected != 1 {
		t.Fatalf("summary is accepted=%d rejected=%d, want 2/1", summary.Accepted, summary.Rejected)
	}
	assertNothingPublished(t, fixture)
}

// exportFixtureLine returns one serialized call record without its newline so
// a table literal can splice a mutation into it before any subtest runs.
func exportFixtureLine() []byte {
	body, err := MarshalRecordSet([]Record{callFixture(1)})
	if err != nil {
		panic("marshal export fixture record")
	}
	return body[:len(body)-1]
}

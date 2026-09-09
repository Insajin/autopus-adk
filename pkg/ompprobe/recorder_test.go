package ompprobe

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRecorder_WritesPrivateJSONLTheExporterAccepts(t *testing.T) {
	directory := filepath.Join(realTempDir(t), "probe")
	recorder, err := NewRecorder(directory)
	if err != nil {
		t.Fatalf("create recorder: %v", err)
	}
	if err := recorder.Record(callFixture(1)); err != nil {
		t.Fatalf("record call: %v", err)
	}
	if err := recorder.Record(compactionFixture(2)); err != nil {
		t.Fatalf("record compaction: %v", err)
	}
	if err := recorder.Close(); err != nil {
		t.Fatalf("close recorder: %v", err)
	}
	if recorder.Records() != 2 {
		t.Fatalf("recorder counted %d records, want 2", recorder.Records())
	}

	directoryInfo, err := os.Lstat(directory)
	if err != nil {
		t.Fatalf("stat probe directory: %v", err)
	}
	if !directoryInfo.IsDir() || directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("probe directory mode is %v, want drwx------", directoryInfo.Mode())
	}
	fileInfo, err := os.Lstat(recorder.Path())
	if err != nil {
		t.Fatalf("stat record file: %v", err)
	}
	if !fileInfo.Mode().IsRegular() || fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("record file mode is %v, want -rw-------", fileInfo.Mode())
	}
	if filepath.Base(recorder.Path()) != RecordFileName {
		t.Fatalf("record file is %s, want %s", recorder.Path(), RecordFileName)
	}

	body, err := os.ReadFile(recorder.Path())
	if err != nil {
		t.Fatalf("read record file: %v", err)
	}
	if !bytes.Equal(body, fixtureBody(t)) {
		t.Fatal("written records differ from the canonical serialization")
	}
	set, err := ParseRecordSet(body)
	if err != nil || set.Accepted != 2 || set.Rejected != 0 {
		t.Fatalf("exporter refused recorder output: err=%v accepted=%d rejected=%d",
			err, set.Accepted, set.Rejected)
	}
}

func TestRecorder_RefusesAnExistingRecordFileAndASymlinkedDirectory(t *testing.T) {
	base := realTempDir(t)
	directory := filepath.Join(base, "probe")
	first, err := NewRecorder(directory)
	if err != nil {
		t.Fatalf("create recorder: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if _, err := NewRecorder(directory); err == nil {
		t.Fatal("a second recorder must not open an existing record file")
	}

	target := filepath.Join(base, "target")
	makeFixtureDirectory(t, target)
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if _, err := NewRecorder(link); err != errRecorderDirectory {
		t.Fatalf("symlinked probe directory error is %v, want %v", err, errRecorderDirectory)
	}
	if _, err := os.Lstat(filepath.Join(target, RecordFileName)); !os.IsNotExist(err) {
		t.Fatal("recorder wrote through a symlinked probe directory")
	}
	if _, err := NewRecorder(""); err != errRecorderDirectory {
		t.Fatal("an empty probe directory must be refused")
	}
}

func TestRecorder_RejectsInvalidRecordsAndEnforcesItsBudget(t *testing.T) {
	recorder, err := NewRecorder(filepath.Join(realTempDir(t), "probe"))
	if err != nil {
		t.Fatalf("create recorder: %v", err)
	}
	invalid := callFixture(1)
	invalid.Roles = map[string]int{"Opaque-Item!": 1}
	if err := recorder.Record(invalid); err != errRecordIdentifier {
		t.Fatalf("invalid record error is %v, want %v", err, errRecordIdentifier)
	}
	for index := range MaxRecords {
		if err := recorder.Record(callFixture(index + 1)); err != nil {
			t.Fatalf("record %d refused below the budget: %v", index, err)
		}
	}
	if err := recorder.Record(callFixture(1)); err != errRecorderBudget {
		t.Fatalf("record past the budget returned %v, want %v", err, errRecorderBudget)
	}
	if err := recorder.Close(); err != nil {
		t.Fatalf("close recorder: %v", err)
	}
	if err := recorder.Record(callFixture(1)); err != errRecorderClosed {
		t.Fatalf("record after close returned %v, want %v", err, errRecorderClosed)
	}
	if err := recorder.Close(); err != nil {
		t.Fatalf("repeated close returned %v, want nil", err)
	}

	body, err := os.ReadFile(recorder.Path())
	if err != nil {
		t.Fatalf("read record file: %v", err)
	}
	set, err := ParseRecordSet(body)
	if err != nil {
		t.Fatalf("parse recorded budget: %v", err)
	}
	if set.Accepted != MaxRecords || set.Rejected != 0 {
		t.Fatalf("file holds accepted=%d rejected=%d, want %d/0", set.Accepted, set.Rejected, MaxRecords)
	}
}

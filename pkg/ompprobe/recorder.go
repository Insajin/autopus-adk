package ompprobe

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var (
	errRecorderDirectory = errors.New("probe directory is unsafe")
	errRecorderCreate    = errors.New("create probe record file")
	errRecorderClosed    = errors.New("probe recorder is closed")
	errRecorderEncode    = errors.New("encode probe record")
	errRecorderBudget    = errors.New("probe record budget is exhausted")
	errRecorderWrite     = errors.New("write probe record")
)

// Recorder appends validated probe records as JSONL under a private
// directory. It is the only writer of RecordFileName and it never appends to
// an existing file: the record file is created exclusively so a pre-planted
// path or a second recorder fails instead of mixing bytes.
//
// A Recorder is not safe for concurrent use; the probe writes from the single
// goroutine that drives the cohort.
type Recorder struct {
	file     *os.File
	path     string
	records  int
	written  int64
	finished bool
}

// NewRecorder prepares dir with mode 0700 and creates a new 0600 record file
// inside it. An existing private directory is accepted, an existing record
// file is not.
func NewRecorder(dir string) (*Recorder, error) {
	if dir == "" {
		return nil, errRecorderDirectory
	}
	resolved, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return nil, errRecorderDirectory
	}
	if err := prepareRecorderDirectory(resolved); err != nil {
		return nil, err
	}
	path := filepath.Join(resolved, RecordFileName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, errRecorderCreate
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != 0 {
		_ = file.Close()
		return nil, errRecorderCreate
	}
	return &Recorder{file: file, path: path}, nil
}

// prepareRecorderDirectory creates dir when absent and asserts that the final
// component is a real private directory. Lstat, not Stat, so a symlink placed
// where the probe directory belongs is refused rather than followed.
func prepareRecorderDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errRecorderDirectory
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return errRecorderDirectory
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return errRecorderDirectory
		}
		if info, err = os.Lstat(dir); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			return errRecorderDirectory
		}
	}
	return nil
}

// Path returns the record file the probe is writing. It tolerates a nil
// receiver so a caller can report the path on an aborted setup path.
func (recorder *Recorder) Path() string {
	if recorder == nil {
		return ""
	}
	return recorder.path
}

// Record validates and appends one record. It fails closed once any bound is
// reached: a truncated line is never written, because the encoded record is
// measured before the write.
func (recorder *Recorder) Record(record Record) error {
	if recorder == nil || recorder.finished || recorder.file == nil {
		return errRecorderClosed
	}
	if err := Validate(record); err != nil {
		return err
	}
	body, err := json.Marshal(record)
	if err != nil {
		return errRecorderEncode
	}
	line := append(body, '\n')
	if len(line) > MaxRecordBytes || recorder.records+1 > MaxRecords ||
		recorder.written+int64(len(line)) > MaxTotalBytes {
		return errRecorderBudget
	}
	count, err := recorder.file.Write(line)
	if err != nil || count != len(line) {
		return errRecorderWrite
	}
	recorder.records++
	recorder.written += int64(count)
	return nil
}

// Records returns how many records were appended.
func (recorder *Recorder) Records() int {
	if recorder == nil {
		return 0
	}
	return recorder.records
}

// Close flushes and releases the record file. Repeated calls are no-ops so the
// probe can close on both the success and the abort path.
func (recorder *Recorder) Close() error {
	if recorder == nil || recorder.finished {
		return nil
	}
	recorder.finished = true
	if recorder.file == nil {
		return nil
	}
	syncErr := recorder.file.Sync()
	closeErr := recorder.file.Close()
	recorder.file = nil
	if syncErr != nil || closeErr != nil {
		return errRecorderWrite
	}
	return nil
}

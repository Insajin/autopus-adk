package cli

import (
	"bytes"
	"encoding/json"
)

const (
	maxBlobEntries           = 10_000
	maxBlobEntryBytes        = 64 << 20
	maxBlobTotalBytes        = 256 << 20
	maxBlobEvents            = 100_000
	maxBlobLineBytes         = 4 << 20
	maxBlobArchiveBytes      = 128 << 20
	maxPlaywrightOutputBytes = 64 << 20
	maxPlaywrightStderrBytes = 4 << 20
)

type boundedOutput struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (output *boundedOutput) Write(data []byte) (int, error) {
	written := len(data)
	remaining := output.limit - output.Len()
	if remaining > 0 {
		if remaining < len(data) {
			data = data[:remaining]
		}
		_, _ = output.Buffer.Write(data)
	}
	if written > remaining {
		output.overflow = true
	}
	return written, nil
}

type blobEvent struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type blobTestInfo struct {
	Project     string
	SnapshotDir string
}

type blobStep struct {
	TestID    string
	ResultID  string
	Name      string
	Anonymous bool
	Ended     bool
	Failed    bool
}

type blobResult struct {
	TestID             string
	ResultID           string
	Retry              int
	Status             string
	Ended              bool
	Order              int
	AnonymousSnapshots int
	Attachments        []playwrightAttachment
}

type blobCollector struct {
	rootDir         string
	tests           map[string]blobTestInfo
	results         map[string]*blobResult
	resultSequence  []*blobResult
	stepsByResult   map[string][]*blobStep
	stepsByID       map[string]map[string]*blobStep
	entries         map[string]struct{}
	projectSet      map[string]struct{}
	resultOrder     int
	snapshotProof   snapshotComparisonProof
	proofPresent    bool
	proofDiagnostic string
	evidence        visualEvidence
}

type blobSuite struct {
	Entries []blobSuiteEntry `json:"entries"`
}

type blobSuiteEntry struct {
	TestID  string           `json:"testId"`
	Entries []blobSuiteEntry `json:"entries"`
}

// playwrightResult is a minimal subset of Playwright JSON reporter output.
type playwrightResult struct {
	Suites        []playwrightSuite `json:"suites"`
	SnapshotProof json.RawMessage   `json:"_autopusSnapshotComparisonProof"`
}

// playwrightSuite holds test suite data from Playwright JSON output.
type playwrightSuite struct {
	Suites []playwrightSuite `json:"suites"`
	Specs  []playwrightSpec  `json:"specs"`
}

// playwrightSpec holds individual test spec data including attachments.
type playwrightSpec struct {
	ID    string           `json:"id"`
	Tests []playwrightTest `json:"tests"`
}

// playwrightTest holds test result data from a single test run.
type playwrightTest struct {
	ID          string                 `json:"id"`
	TestID      string                 `json:"testId"`
	ProjectName string                 `json:"projectName"`
	Results     []playwrightTestResult `json:"results"`
}

// playwrightTestResult holds attachments such as screenshot paths.
type playwrightTestResult struct {
	ID          string                 `json:"id"`
	Retry       int                    `json:"retry"`
	Status      string                 `json:"status"`
	Attachments []playwrightAttachment `json:"attachments"`
}

// playwrightAttachment represents a file attachment (e.g., screenshot) in a test result.
type playwrightAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
}

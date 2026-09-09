package ompprobe

import (
	"errors"
	"path/filepath"
	"strings"
)

const maxSourceDepth = 8

var errExportOptions = errors.New("probe export arguments are unsafe")

// ExportOptions describes one export of a probe record file out of the
// disposable canary workspace into a retained directory.
//
// SourceRoot is untrusted: it is the isolated workspace a canary UID owned, so
// every component of SourceRoot and SourceRelative is traversed without
// following symlinks and the source file is accepted only on its own
// descriptor. DestinationRoot is the trusted retained directory; the exporter
// creates DestinationName inside it exclusively and never overwrites.
type ExportOptions struct {
	SourceRoot      string
	SourceRelative  string
	DestinationRoot string
	DestinationName string
	SourceUID       uint32
	OwnerUID        uint32
	OwnerGID        uint32
}

// ExportSummary is the body-free result reported to the release lane. The
// counts and Complete describe the records actually validated and retained,
// so a lane that reads Complete is not trusting the probe's own completion
// frame: a truncated or repeated schedule reports the same totals as a whole
// cohort but is never complete.
type ExportSummary struct {
	Accepted          int  `json:"accepted"`
	Rejected          int  `json:"rejected"`
	CallRecords       int  `json:"call_records"`
	CompactionRecords int  `json:"compaction_records"`
	Complete          bool `json:"complete"`
}

// exportPlan is the validated split of SourceRelative into the directories to
// descend and the single file to open.
type exportPlan struct {
	directories []string
	file        string
}

// Export reads, validates and republishes probe records. Nothing is created at
// the destination until every record has been decoded and validated in memory,
// and the bytes published are re-serialized from those records rather than
// copied from the source.
func Export(options ExportOptions) (ExportSummary, error) {
	var summary ExportSummary
	plan, err := options.plan()
	if err != nil {
		return summary, err
	}
	body, err := readProbeSource(options, plan)
	if err != nil {
		return summary, err
	}
	set, parseErr := ParseRecordSet(body)
	summary.Accepted, summary.Rejected = set.Accepted, set.Rejected
	summary.CallRecords, summary.CompactionRecords = set.CallRecords, set.CompactionRecords
	clear(body)
	if parseErr != nil {
		return summary, parseErr
	}
	payload, err := MarshalRecordSet(set.Records)
	if err != nil {
		return summary, err
	}
	if err := publishProbeRecords(options, payload); err != nil {
		return summary, err
	}
	// Only now is the cohort retained, so only now may the lane be told it is
	// complete: a refused export accounts for what it read and nothing more.
	summary.Complete = set.Complete
	return summary, nil
}

func (options ExportOptions) plan() (exportPlan, error) {
	if !validRootPath(options.SourceRoot) || !validRootPath(options.DestinationRoot) ||
		!validDestinationName(options.DestinationName) {
		return exportPlan{}, errExportOptions
	}
	relative := options.SourceRelative
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != relative {
		return exportPlan{}, errExportOptions
	}
	components := strings.Split(relative, string(filepath.Separator))
	if len(components) > maxSourceDepth {
		return exportPlan{}, errExportOptions
	}
	for _, component := range components {
		if !validRelativeComponent(component) {
			return exportPlan{}, errExportOptions
		}
	}
	return exportPlan{
		directories: components[:len(components)-1],
		file:        components[len(components)-1],
	}, nil
}

// validRootPath accepts an absolute, already-clean path below the filesystem
// root. Component names are checked during traversal, not here: the security
// property is that no component is followed as a symlink, not that operators
// keep their checkout under a restricted character set.
func validRootPath(path string) bool {
	if len(path) < 2 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	for _, component := range strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator)) {
		if !validRootComponent(component) {
			return false
		}
	}
	return true
}

func validRootComponent(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 255 {
		return false
	}
	return !strings.ContainsAny(name, "/\x00")
}

// validRelativeComponent is stricter than validRootComponent because the
// relative source path is chosen by this repository, not by the operator.
func validRelativeComponent(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for index := range len(name) {
		character := name[index]
		switch {
		case character >= 'A' && character <= 'Z', character >= 'a' && character <= 'z',
			character >= '0' && character <= '9':
		case index > 0 && (character == '.' || character == '_' || character == '-'):
		default:
			return false
		}
	}
	return true
}

// validDestinationName pins the retained file to the probe-<nonce>.jsonl shape
// the release lane publishes.
func validDestinationName(name string) bool {
	const prefix, suffix = "probe-", ".jsonl"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return false
	}
	nonce := name[len(prefix) : len(name)-len(suffix)]
	if len(nonce) == 0 || len(nonce) > 64 {
		return false
	}
	for index := range len(nonce) {
		character := nonce[index]
		switch {
		case character >= 'A' && character <= 'Z', character >= 'a' && character <= 'z',
			character >= '0' && character <= '9':
		case index > 0 && (character == '_' || character == '-'):
		default:
			return false
		}
	}
	return true
}

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
)

// readinessIndexSource describes one evidence index `auto qa readiness` resolves
// by default, together with the command that produces it and the flag that
// overrides the default.
type readinessIndexSource struct {
	Kind     string
	RelDir   string
	FileName string
	Flag     string
	Producer string
}

var (
	readinessRunIndexSource = readinessIndexSource{
		Kind:     "run",
		RelDir:   filepath.Join(".autopus", "qa", "runs"),
		FileName: "run-index.json",
		Flag:     "run-index",
		Producer: "auto qa run",
	}
	readinessReleaseIndexSource = readinessIndexSource{
		Kind:     "release",
		RelDir:   filepath.Join(".autopus", "qa", "releases"),
		FileName: "release-index.json",
		Flag:     "release-index",
		Producer: "auto qa release",
	}
)

// readinessMissingEvidenceError reports that a default-resolved index does not
// exist yet. It is distinct from a bad flag: the caller's arguments were fine,
// the evidence simply has not been produced, so the message names the command
// that produces it instead of blaming the invocation.
type readinessMissingEvidenceError struct {
	source readinessIndexSource
	dir    string
}

func (e readinessMissingEvidenceError) Error() string {
	return fmt.Sprintf("no QAMESH %s index under %s; run `%s` first, or pass --%s",
		e.source.Kind, e.dir, e.source.Producer, e.source.Flag)
}

// requiredLatestIndexPath resolves the newest index of a source and fails with
// an actionable error when none exists. latestIndexPath reports "no match" as an
// empty path, which downstream turned into a path-less `open :` error.
func requiredLatestIndexPath(workspaceRoot string, source readinessIndexSource) (string, error) {
	path, err := latestIndexPath(workspaceRoot, source.RelDir, source.FileName)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", readinessMissingEvidenceError{source: source, dir: filepath.Join(workspaceRoot, source.RelDir)}
	}
	return path, nil
}

// readinessDefaultsErrorCode keeps qa_readiness_invalid_flags for genuine flag
// problems so a missing-evidence exit is not misread as operator error.
func readinessDefaultsErrorCode(err error) string {
	var missing readinessMissingEvidenceError
	if errors.As(err, &missing) {
		return "qa_readiness_no_evidence"
	}
	return "qa_readiness_invalid_flags"
}

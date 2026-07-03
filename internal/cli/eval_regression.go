package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/evalregression"
)

// Load-failure reason codes emitted by the CLI loader before the pure gate
// runs. These mirror the machine-readable reason literals asserted by the
// deterministic oracles (G3, G4, G6). The pure gate emits the verdict-path
// reason codes (regression_blocked / ok / artifact_stale / artifact_unsafe /
// artifact_invalid); these three cover the pre-decode load failures.
const (
	reasonArtifactMissing = "artifact_missing"
	reasonArtifactInvalid = "artifact_invalid"
	reasonArtifactUnsafe  = "artifact_unsafe"
)

// loadEvalRegressionReport reads the eval_regression_report.v1 artifact at path
// and strictly decodes it. It mirrors the file-handling of
// internal/cli/qa_coverage.go loadCoverageJSON (os.ReadFile + absent-file
// handling), BUT decodes with a strict json.Decoder on which
// DisallowUnknownFields() is called before Decode — NOT a plain json.Unmarshal,
// which silently drops unknown fields.
//
// On failure it returns a machine-readable reason string alongside the error:
//   - absent file            → reason artifact_missing (fail closed, REQ-ECI-MISSING-001)
//   - unknown/out-of-schema field → reason artifact_unsafe (REQ-ECI-STRICT-001):
//     an out-of-schema raw-payload field must NOT be silently dropped, so it can
//     never reach the verdict path even when RawPayloadPresent is false.
//   - any other JSON syntax/type error → reason artifact_invalid (REQ-ECI-INVALID-001)
func loadEvalRegressionReport(path string) (evalregression.EvalRegressionReportV1, string, error) {
	var report evalregression.EvalRegressionReportV1

	if strings.TrimSpace(path) == "" {
		return report, reasonArtifactMissing, fmt.Errorf("eval-regression artifact path is empty")
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return report, reasonArtifactMissing, fmt.Errorf("eval-regression artifact not found")
	}
	if err != nil {
		return report, reasonArtifactInvalid, err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&report); err != nil {
		// DisallowUnknownFields surfaces an out-of-schema field as an error whose
		// message contains "unknown field"; treat that as an unsafe artifact so
		// the raw-payload field never reaches the verdict path.
		if strings.Contains(err.Error(), "unknown field") {
			return report, reasonArtifactUnsafe, err
		}
		return report, reasonArtifactInvalid, err
	}

	// Reject trailing content after the object: a valid object followed by extra
	// bytes (e.g. `{...}{"x":1}`) is malformed input and must fail closed rather
	// than silently accepting only the leading value (REQ-ECI-INVALID-001).
	if _, err := dec.Token(); err != io.EOF {
		return report, reasonArtifactInvalid, fmt.Errorf("trailing data after JSON object")
	}

	return report, "", nil
}

// checkEvalRegression loads the artifact, evaluates the deterministic gate with
// the injected clock, prints a redacted one-line verdict, and returns true
// (pass) iff the gate exit code is zero. On any load failure it builds a
// fail-closed GateDecision from the load reason (Blocked, exit 1).
//
// The printed line carries only the machine-readable reason code plus the
// gate-sanitized attributed version — the pure gate already allowlist-sanitizes
// AttributedVersion (REQ-ECI-SANITIZE-001), so no raw artifact string is ever
// echoed into the retained CI log.
//
// WHERE warnOnly is set the function always returns true (advisory) but still
// prints the verdict (REQ-ECI-WARN-001).
func checkEvalRegression(dir, artifactPath string, maxAge time.Duration, now time.Time, out io.Writer, quiet, warnOnly bool) bool {
	_ = dir // artifact path is absolute/explicit; dir is accepted for dispatch symmetry.

	report, loadReason, loadErr := loadEvalRegressionReport(artifactPath)

	var decision evalregression.GateDecision
	if loadErr != nil {
		decision = evalregression.GateDecision{Blocked: true, ExitCode: 1, Reason: loadReason}
	} else {
		decision = evalregression.EvaluateEvalRegressionGate(report, now, maxAge)
	}

	if !quiet {
		line := "eval-regression: " + decision.Reason
		if decision.AttributedVersion != "" {
			line += " (version=" + decision.AttributedVersion + ")"
		}
		if warnOnly && decision.ExitCode != 0 {
			line += " [advisory: warn-only]"
		}
		fmt.Fprintln(out, line)
	}

	if warnOnly {
		return true
	}
	return decision.ExitCode == 0
}

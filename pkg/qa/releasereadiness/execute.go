package releasereadiness

import (
	"encoding/json"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/release"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

// runOutputDir is the hermetic per-run artifact root under the project's
// .autopus tree. qarun writes run-index and manifests beneath it.
func runOutputDir(projectDir string) string {
	return filepath.Join(projectDir, ".autopus", "qa", "_release_readiness")
}

// mapRunStatusToLane maps a qarun Result.Status to a release.LaneStatus value,
// matching pkg/qa/release/execute.go mapRunStatus semantics: warning->warn,
// passed/failed/blocked pass through, anything else (including the empty string
// or "setup-gap") fails closed to blocked.
func mapRunStatusToLane(status string) release.LaneStatus {
	switch status {
	case "warning":
		return release.LaneStatusWarn
	case "passed", "failed", "blocked":
		return release.LaneStatus(status)
	default:
		return release.LaneStatusBlocked
	}
}

// laneRowFromRun fills a LaneRow from a qarun result. A runner error with a
// non-failing status is treated as blocked so a run failure never reads as a
// pass. DeterministicAuthority stays true: verdicts derive from exit codes and
// deterministic checks, never from AI analysis.
func laneRowFromRun(row LaneRow, result qarun.Result, err error) LaneRow {
	status := mapRunStatusToLane(result.Status)
	if err != nil && status != release.LaneStatusFailed && status != release.LaneStatusBlocked {
		status = release.LaneStatusBlocked
	}
	if status == release.LaneStatusPassed && desktopObservationDispatch(row, result) &&
		!validPassingDesktopObservation(result) {
		status = release.LaneStatusBlocked
	}
	row.Status = string(status)
	if fs := firstFailureSummary(result); fs != "" {
		row.FailureSummary = fs
	}
	return row
}

func desktopObservationDispatch(row LaneRow, result qarun.Result) bool {
	if row.adapterID != "" {
		return row.adapterID == "desktop-accessibility-observe"
	}
	for _, adapterResult := range result.AdapterResults {
		if adapterResult.Adapter == "desktop-accessibility-observe" {
			return true
		}
	}
	for _, check := range result.Checks {
		if check.Adapter == "desktop-accessibility-observe" || check.ID == "desktop-semantic-landmarks" {
			return true
		}
	}
	return row.Lane == "desktop-native"
}

func validPassingDesktopObservation(result qarun.Result) bool {
	if result.RedactionStatus.Status != "passed" {
		return false
	}
	adapterPassed := false
	for _, adapterResult := range result.AdapterResults {
		if adapterResult.Adapter != "desktop-accessibility-observe" {
			continue
		}
		if adapterResult.Status != "passed" || adapterResult.SetupGap != nil ||
			adapterResult.DesktopObservation == nil || adapterResult.DesktopObservation.RuntimeReceipt.Validate() != nil {
			return false
		}
		body, err := json.Marshal(adapterResult.DesktopObservation)
		if err != nil {
			return false
		}
		if _, err := desktopobserve.DecodeObservationEvidence(body); err != nil {
			return false
		}
		adapterPassed = true
	}
	checkPassed := false
	for _, check := range result.Checks {
		if check.Adapter != "desktop-accessibility-observe" && check.ID != "desktop-semantic-landmarks" {
			continue
		}
		if check.Status != "passed" {
			return false
		}
		checkPassed = true
	}
	return adapterPassed && checkPassed
}

func firstFailureSummary(result qarun.Result) string {
	for _, ar := range result.AdapterResults {
		if ar.FailureSummary != "" {
			return ar.FailureSummary
		}
	}
	return ""
}

// aggregateVerdict normalizes each lane row into a release.LaneRow under
// LanePolicyMust and aggregates a deterministic gate status. With LanePolicyMust
// any failed/blocked/setup-gap lane forces a non-passing verdict, so a setup gap
// can never be laundered into a pass.
func aggregateVerdict(rows []LaneRow) Verdict {
	releaseRows := make([]release.LaneRow, 0, len(rows))
	for _, row := range rows {
		rr := release.LaneRow{
			Lane:                   row.Lane,
			LanePolicy:             release.LanePolicyMust,
			Status:                 release.LaneStatus(row.Status),
			DeterministicAuthority: true,
		}
		if row.ReasonCode != "" {
			rr.SetupGapClass = release.SetupGapToolUnavailable
		}
		releaseRows = append(releaseRows, release.NormalizeLaneRow(rr))
	}
	gate := release.AggregateGateStatus(releaseRows)
	return Verdict{Status: string(gate), DeterministicAuthority: true}
}

// buildEvidenceSummary returns a sanitized release-readiness summary manifest as
// a serialized JSON string. It declares schema_version qamesh.evidence.v2,
// passes the bytes through RedactText, and asserts AssertSafeText so no raw
// secret, device handle, media, or local path is ever published (AC-010). The
// summary is published in-payload rather than written to disk, which avoids the
// empty-output-dir and non-empty-artifacts requirements of WriteFinalManifest
// while still binding the v2 schema contract and the safe-text gate.
func buildEvidenceSummary(payload Payload) (string, error) {
	summary := struct {
		SchemaVersion    string     `json:"schema_version"`
		AnalyzedSurfaces []string   `json:"analyzed_surfaces"`
		Phase            string     `json:"phase"`
		FilesWritten     int        `json:"files_written"`
		LanesExecuted    int        `json:"lanes_executed"`
		LaneRows         []LaneRow  `json:"lane_rows"`
		Verdict          Verdict    `json:"verdict"`
		Diff             diffCounts `json:"diff_counts"`
	}{
		SchemaVersion:    qaevidence.SchemaVersionV2,
		AnalyzedSurfaces: payload.AnalyzedSurfaces,
		Phase:            payload.Phase,
		FilesWritten:     payload.FilesWritten,
		LanesExecuted:    payload.LanesExecuted,
		LaneRows:         payload.LaneRows,
		Verdict:          payload.Verdict,
		Diff: diffCounts{
			Added:   payload.Diff.AddedCount,
			Changed: payload.Diff.ChangedCount,
			Removed: payload.Diff.RemovedCount,
		},
	}
	body, err := json.Marshal(summary)
	if err != nil {
		return "", err
	}
	text := qaevidence.RedactText(string(body))
	if err := qaevidence.AssertSafeText(text, "release_readiness.evidence_summary"); err != nil {
		return "", err
	}
	return text, nil
}

type diffCounts struct {
	Added   int `json:"added"`
	Changed int `json:"changed"`
	Removed int `json:"removed"`
}

// Package releasereadiness orchestrates the release-time cross-surface Journey
// Pack regeneration flow with an explicit diff-approval gate. It composes the
// Unit 1 regen pipeline (analyze -> synthesize -> validate -> ai-guard -> diff),
// owns the approval gate (no write or execution before --approve), and on
// approval persists accepted packs and dispatches per-surface lanes through
// pkg/qa/run, aggregating a deterministic verdict via pkg/qa/release.
//
// The package registers no scheduler, hook, or CI auto-trigger: the only entry
// point is Orchestrate, invoked by the explicit CLI command (Unit 3). There is
// deliberately no init() or background trigger here (AC-006).
package releasereadiness

import "github.com/insajin/autopus-adk/pkg/qa/regen"

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-011: SchemaVersion is the published payload envelope discriminator — bumping it is a breaking change for any consumer parsing the JSON output.
// SchemaVersion is the release-readiness payload envelope version.
const SchemaVersion = "qamesh.release_readiness.v1"

// PhaseStatus enumerates the distinct lifecycle phases of an orchestration run.
type PhaseStatus string

const (
	// PhaseAnalyzed is reported when no surfaces are present, so there is
	// nothing to regenerate and no false "regenerated" claim is made.
	PhaseAnalyzed PhaseStatus = "analyzed"
	// PhaseDiffPresented is reported when surfaces were analyzed and a diff was
	// produced but approval was not granted, so nothing is written or executed.
	PhaseDiffPresented PhaseStatus = "diff_presented"
	// PhaseApproved is an intermediate marker; Orchestrate returns PhaseExecuted
	// after a successful approved run. Retained for contract completeness.
	PhaseApproved PhaseStatus = "approved"
	// PhaseExecuted is reported after approved persistence and lane dispatch.
	PhaseExecuted PhaseStatus = "executed"
	// PhaseDeclined is reported when the operator explicitly declines approval.
	PhaseDeclined PhaseStatus = "declined"
)

// Options drives a single orchestration run. Approve and Decline are mutually
// exclusive operator signals; Decline takes precedence when both are set so a
// decline never produces side effects.
type Options struct {
	ProjectDir string `json:"project_dir"`
	Approve    bool   `json:"approve"`
	Decline    bool   `json:"decline"`
}

// LaneRow is the release-readiness view of one dispatched (or gap) lane. Status
// holds a release.LaneStatus string value (passed|failed|setup_gap|...).
// ReasonCode carries a surface-dispatch reason code when the lane was a
// setup-gap (surface_tool_unavailable|surface_absent).
type LaneRow struct {
	Lane                   string `json:"lane"`
	Status                 string `json:"status"`
	ReasonCode             string `json:"reason_code,omitempty"`
	FailureSummary         string `json:"failure_summary,omitempty"`
	DeterministicAuthority bool   `json:"deterministic_authority"`
}

// Verdict is the aggregated deterministic gate decision over all lane rows.
type Verdict struct {
	Status                 string `json:"status"`
	DeterministicAuthority bool   `json:"deterministic_authority"`
}

// Payload is the full serialized release-readiness result the CLI emits.
type Payload struct {
	SchemaVersion    string     `json:"schema_version"`
	AnalyzedSurfaces []string   `json:"analyzed_surfaces"`
	Phase            string     `json:"phase"`
	Diff             regen.Diff `json:"diff"`
	FilesWritten     int        `json:"files_written"`
	LanesExecuted    int        `json:"lanes_executed"`
	LaneRows         []LaneRow  `json:"lane_rows"`
	Verdict          Verdict    `json:"verdict"`
	EvidenceSummary  string     `json:"evidence_summary,omitempty"`
}

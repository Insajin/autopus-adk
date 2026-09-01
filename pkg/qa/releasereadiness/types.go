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

import (
	"errors"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	"github.com/insajin/autopus-adk/pkg/qa/regen"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-011: SchemaVersion is the published payload envelope discriminator — bumping it is a breaking change for any consumer parsing the JSON output.
// SchemaVersion is the release-readiness payload envelope version. v2 renamed
// the diff's "removed" category to "unmatched", added approval_deletes_packs,
// and replaced the unconditional passed verdict with VerdictNotEvaluated on
// every phase that dispatched no lane. A v1 consumer would misread all three.
const SchemaVersion = "qamesh.release_readiness.v2"

// VerdictNotEvaluated is the verdict status reported whenever no lane was
// dispatched. It is deliberately NOT a release.GateStatus value: no
// deterministic gate ran, so no consumer may read the result as a pass.
const VerdictNotEvaluated = "not_evaluated"

// ErrApprovalIntentConflict rejects Approve together with Decline. Silently
// resolving a precedence between two contradictory operator intents hides which
// one the harness obeyed, so the run is refused instead.
var ErrApprovalIntentConflict = errors.New("approve and decline are mutually exclusive")

// PhaseStatus enumerates the distinct lifecycle phases of an orchestration run.
type PhaseStatus string

const (
	// PhaseAnalyzed is reported when the run had nothing to regenerate and
	// nothing to dispatch, so no "regenerated" or "executed" claim is made. It
	// covers both the unapproved no-surface case and an approved run that
	// dispatched zero lanes.
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
// exclusive operator signals; setting both is refused with
// ErrApprovalIntentConflict rather than resolved by precedence.
type Options struct {
	ProjectDir      string                         `json:"project_dir"`
	Approve         bool                           `json:"approve"`
	Decline         bool                           `json:"decline"`
	RuntimeProvider desktopobserve.RuntimeProvider `json:"runtime_provider,omitempty"`
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
	adapterID              string
}

// Verdict is the aggregated deterministic gate decision over the lane rows.
// DeterministicAuthority is true only when at least one lane actually ran and
// the status was derived from its exit-code evidence; a run that dispatched
// nothing reports VerdictNotEvaluated with authority false.
type Verdict struct {
	Status                 string `json:"status"`
	DeterministicAuthority bool   `json:"deterministic_authority"`
}

// Payload is the full serialized release-readiness result the CLI emits.
//
// ApprovalDeletesPacks is always false and is published as an explicit fact so
// no consumer has to infer non-destructiveness from the shape of the diff:
// approval writes accepted packs and never deletes an existing one.
type Payload struct {
	SchemaVersion        string     `json:"schema_version"`
	AnalyzedSurfaces     []string   `json:"analyzed_surfaces"`
	Phase                string     `json:"phase"`
	Diff                 regen.Diff `json:"diff"`
	ApprovalDeletesPacks bool       `json:"approval_deletes_packs"`
	FilesWritten         int        `json:"files_written"`
	LanesExecuted        int        `json:"lanes_executed"`
	LaneRows             []LaneRow  `json:"lane_rows"`
	Verdict              Verdict    `json:"verdict"`
	EvidenceSummary      string     `json:"evidence_summary,omitempty"`
}

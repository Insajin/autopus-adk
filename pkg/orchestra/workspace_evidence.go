package orchestra

import (
	"fmt"
	"strings"
)

// Workspace evidence status vocabulary.
const (
	WorkspaceStatusClean       = "clean"
	WorkspaceStatusMutated     = "mutated"
	WorkspaceStatusUnavailable = "unavailable"
)

// DegradedReasonWorkspaceMutation is the machine-readable failure code recorded
// when provider execution changed the caller's worktree.
const DegradedReasonWorkspaceMutation = "workspace_mutation_detected"

// WorkspaceSnapshot summarizes one read-only porcelain status capture.
type WorkspaceSnapshot struct {
	Entries int    `json:"entries"`
	SHA256  string `json:"sha256"`
}

// WorkspaceEvidence records the pre/post provider-execution worktree comparison.
// Status is clean, mutated, or unavailable (the root is not a git worktree).
type WorkspaceEvidence struct {
	Root             string             `json:"root"`
	SnapshotBefore   *WorkspaceSnapshot `json:"snapshot_before,omitempty"`
	SnapshotAfter    *WorkspaceSnapshot `json:"snapshot_after,omitempty"`
	MutationDetected bool               `json:"mutation_detected"`
	ChangedFiles     []string           `json:"changed_files,omitempty"`
	Status           string             `json:"status"`
}

// WorkspaceMutationError is the surfaced failure when provider execution
// changed the caller's worktree. It takes precedence over the run's own
// failure, which stays reachable through Unwrap and the diagnostics artifact.
// No rollback is performed: the changed paths are reported for the caller.
type WorkspaceMutationError struct {
	ChangedFiles []string
	Cause        error
}

func (e *WorkspaceMutationError) Error() string {
	message := fmt.Sprintf("%s: provider execution modified the caller's worktree (%d files): %s",
		DegradedReasonWorkspaceMutation, len(e.ChangedFiles), strings.Join(e.ChangedFiles, ", "))
	if e.Cause != nil {
		message += "; original failure: " + e.Cause.Error()
	}
	return message
}

func (e *WorkspaceMutationError) Unwrap() error {
	return e.Cause
}

// ApplyWorkspaceEvidence attaches the worktree comparison to a finalized result
// and re-projects the receipt. A detected mutation blocks the run: the gate and
// terminal state flip to blocked with the mutation reason recorded, and the
// transition history keeps the prior terminal state so the original outcome
// (for example a provider timeout) remains visible next to the new one.
func ApplyWorkspaceEvidence(result *OrchestraResult, evidence WorkspaceEvidence) {
	if result == nil {
		return
	}
	result.Workspace = &evidence
	var priorTransitions []OrchestrationTransition
	if result.RunReceipt != nil {
		priorTransitions = append(priorTransitions, result.RunReceipt.Transitions...)
	}
	if evidence.MutationDetected {
		result.Degraded = true
		result.TerminalState = TerminalBlocked
		result.GateStatus = "blocked"
		result.AnalysisVerdict = "fail"
		appendDegradedReason(result, DegradedReasonWorkspaceMutation)
	}
	if result.RunReceipt == nil {
		return
	}
	refreshOrchestrationRunReceipt(result)
	if !evidence.MutationDetected || len(priorTransitions) == 0 {
		return
	}
	last := priorTransitions[len(priorTransitions)-1]
	if last.State == result.TerminalState && last.GateStatus == result.GateStatus {
		result.RunReceipt.Transitions = priorTransitions
		return
	}
	result.RunReceipt.Transitions = append(priorTransitions, OrchestrationTransition{
		Sequence: len(priorTransitions) + 1, State: result.TerminalState,
		AnalysisVerdict: result.AnalysisVerdict, GateStatus: result.GateStatus,
	})
}

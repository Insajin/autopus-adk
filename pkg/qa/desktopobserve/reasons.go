package desktopobserve

import (
	"errors"
	"fmt"
)

type ReasonCode string

const (
	ReasonProviderUnavailable            ReasonCode = "provider_unavailable"
	ReasonCapabilityUnsupported          ReasonCode = "capability_unsupported"
	ReasonAccessibilityPermissionMissing ReasonCode = "accessibility_permission_missing"
	ReasonTargetAppNotFound              ReasonCode = "target_app_not_found"
	ReasonTargetWindowNotFound           ReasonCode = "target_window_not_found"
	ReasonStaleState                     ReasonCode = "stale_state"
	ReasonSemanticProjectionUnavailable  ReasonCode = "semantic_projection_unavailable"
	ReasonRedactionFailed                ReasonCode = "redaction_failed"
	ReasonEvidenceQuarantined            ReasonCode = "evidence_quarantined"
	ReasonProviderProtocolMismatch       ReasonCode = "provider_protocol_mismatch"
	// SPEC-QAMESH-013 REQ-5: a landmark the pack declared was absent from the
	// observed tree. Folding this into ReasonProviderUnavailable or
	// ReasonSemanticProjectionUnavailable sent an investigation to the provider
	// lifecycle when the real cause was a window-title mismatch.
	ReasonDeclaredLandmarkNotFound ReasonCode = "declared_landmark_not_found"
	// SPEC-QAMESH-013 REQ-6: the observed tree crossed the node, depth, or byte
	// bound. Distinct from ReasonProviderProtocolMismatch, which is where a size
	// refusal used to land once it degraded into a bare malformed envelope.
	ReasonObservedTreeBoundExceeded ReasonCode = "observed_tree_bound_exceeded"
)

var reasonOrder = []ReasonCode{
	ReasonProviderUnavailable,
	ReasonCapabilityUnsupported,
	ReasonAccessibilityPermissionMissing,
	ReasonTargetAppNotFound,
	ReasonTargetWindowNotFound,
	ReasonStaleState,
	ReasonSemanticProjectionUnavailable,
	ReasonRedactionFailed,
	ReasonEvidenceQuarantined,
	ReasonProviderProtocolMismatch,
	ReasonDeclaredLandmarkNotFound,
	ReasonObservedTreeBoundExceeded,
}

var safeNextSteps = map[ReasonCode]string{
	ReasonProviderUnavailable:            "Check the selected provider lifecycle, then rerun with the same explicit selection.",
	ReasonCapabilityUnsupported:          "Inspect the capability summary and select a supporting provider version explicitly.",
	ReasonAccessibilityPermissionMissing: "Grant Accessibility to the displayed signed identity in Privacy & Security, then rerun.",
	ReasonTargetAppNotFound:              "Start the expected signed app and verify its project-local public alias.",
	ReasonTargetWindowNotFound:           "Open the expected window and verify its project-local public alias.",
	ReasonStaleState:                     "Capture fresh state and evaluate the new state reference exactly once.",
	ReasonSemanticProjectionUnavailable:  "Fix the target surface Accessibility landmarks without using an OCR fallback.",
	ReasonRedactionFailed:                "Keep the payload unpublished and correct the local redaction policy finding.",
	ReasonEvidenceQuarantined:            "Keep raw material local and regenerate a safe semantic projection.",
	ReasonProviderProtocolMismatch:       "Align the explicitly selected provider and adapter protocol versions.",
	ReasonDeclaredLandmarkNotFound:       "Align the declared landmark role and name with the observed surface, then rerun.",
	ReasonObservedTreeBoundExceeded:      "Observe a surface within the declared node, depth, and byte bounds instead of truncating the tree.",
}

func ReasonCodes() []ReasonCode {
	return append([]ReasonCode(nil), reasonOrder...)
}

func NextStep(reason ReasonCode) string {
	return safeNextSteps[reason]
}

func validReason(reason ReasonCode) bool {
	_, ok := safeNextSteps[reason]
	return ok
}

func ReasonCodeOf(err error) ReasonCode {
	var normalized reasonError
	if errors.As(err, &normalized) {
		return normalized.code
	}
	return ""
}

// ObservedTreeBound names which bound a real observed tree crossed. A runtime
// receipt carries only a normalized reason code and that code's fixed next
// step, so the crossed bound has to travel as a typed value; REQ-6 requires the
// refusal to name it.
type ObservedTreeBound string

const (
	ObservedTreeBoundNodes ObservedTreeBound = "node"
	ObservedTreeBoundDepth ObservedTreeBound = "depth"
	ObservedTreeBoundBytes ObservedTreeBound = "byte"
)

type boundedTreeError struct {
	bound    ObservedTreeBound
	limit    int
	observed int
}

func (err boundedTreeError) Error() string {
	return fmt.Sprintf("observed tree exceeds the %s bound: observed %d, allowed %d",
		err.bound, err.observed, err.limit)
}

// Unwrap carries the reason code, so ReasonCodeOf and errors.Is resolve this
// refusal to ReasonObservedTreeBoundExceeded rather than to a generic envelope
// fault.
func (err boundedTreeError) Unwrap() error {
	return reasonError{code: ReasonObservedTreeBoundExceeded}
}

// ObservedTreeBoundExceeded refuses an observation by name. REQ-6 forbids
// truncating or sampling an oversized tree, so a named refusal is the only
// correct outcome; counts are provider-side sizes, never observed content.
func ObservedTreeBoundExceeded(bound ObservedTreeBound, limit, observed int) error {
	return boundedTreeError{bound: bound, limit: limit, observed: observed}
}

type missingLandmarkError struct {
	role Role
	name string
}

func (err missingLandmarkError) Error() string {
	return fmt.Sprintf("declared landmark not found: role %s, name %q", err.role, err.name)
}

func (err missingLandmarkError) Unwrap() error {
	return reasonError{code: ReasonDeclaredLandmarkNotFound}
}

// DeclaredLandmarkNotFound refuses an observation whose declared landmark was
// absent. Role and name are pack-declared, not observed content, so naming them
// keeps the diagnosis actionable without publishing anything from the tree.
func DeclaredLandmarkNotFound(role Role, name string) error {
	return missingLandmarkError{role: role, name: name}
}

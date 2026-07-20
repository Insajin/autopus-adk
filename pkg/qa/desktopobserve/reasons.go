package desktopobserve

import "errors"

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

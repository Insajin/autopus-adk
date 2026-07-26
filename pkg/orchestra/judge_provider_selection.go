package orchestra

import "strings"

const (
	JudgeSelectionExplicit           = "explicit"
	JudgeSelectionInvokingProvider   = "invoking_provider"
	JudgeSelectionConfiguredFallback = "configured_fallback"
)

// JudgeProviderSelectionEvidence records provider-selection provenance only.
// It intentionally excludes process, session, environment, and credential data.
// @AX:ANCHOR: [AUTO] public JSON provenance contract for explicit, invoking-provider, and configured judge selection
// @AX:REASON: [AUTO] run receipts and external diagnostics depend on stable redacted selection-source semantics
type JudgeProviderSelectionEvidence struct {
	InvokingProvider      string `json:"invoking_provider,omitempty"`
	SelectedJudgeProvider string `json:"selected_judge_provider"`
	SelectionSource       string `json:"selection_source"`
	ProviderMatch         *bool  `json:"provider_match,omitempty"`
}

func evaluateJudgeProviderSelection(
	invokingProvider string,
	selectedJudgeProvider string,
	source string,
) *JudgeProviderSelectionEvidence {
	invoker := providerCanonicalName(invokingProvider)
	selected := providerCanonicalName(selectedJudgeProvider)
	evidence := &JudgeProviderSelectionEvidence{
		InvokingProvider:      invoker,
		SelectedJudgeProvider: selected,
		SelectionSource:       normalizeJudgeSelectionSource(source),
	}
	if invoker != "" {
		matches := invoker == selected
		evidence.ProviderMatch = &matches
	}
	return evidence
}

func normalizeJudgeSelectionSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case JudgeSelectionExplicit:
		return JudgeSelectionExplicit
	case JudgeSelectionInvokingProvider:
		return JudgeSelectionInvokingProvider
	case JudgeSelectionConfiguredFallback:
		return JudgeSelectionConfiguredFallback
	default:
		return JudgeSelectionConfiguredFallback
	}
}

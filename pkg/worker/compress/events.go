package compress

import (
	"regexp"
	"sort"
	"strings"
)

const (
	EventTypeCompaction = "context_compaction"

	// @AX:NOTE: [AUTO] @AX:SPEC: SPEC-CONTEXT-COMPRESS-001: reason strings are compaction event wire values consumed by JSONL logs and tests
	ReasonBelowThreshold         = "below_threshold"
	ReasonSoftPrune              = "soft_prune"
	ReasonThresholdExceeded      = "threshold_exceeded"
	ReasonToolPairPruned         = "tool_pair_pruned"
	ReasonIncompleteToolPair     = "incomplete_tool_pair"
	ReasonSecretRedacted         = "secret_redacted"
	ReasonLocalPathRedacted      = "local_path_redacted"
	ReasonProviderPayloadOmitted = "provider_payload_omitted"
	ReasonContextBudgetBlocker   = "context_budget_blocker"
	ReasonIndexEligible          = "index_eligible_redacted_derived_context"
)

var sourceRefPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bSPEC-[A-Z0-9-]+-\d+\b`),
	regexp.MustCompile(`\bAC-[A-Z0-9-]+-\d+\b`),
	regexp.MustCompile(`\bQAMESH-[A-Za-z0-9._-]+\b`),
	regexp.MustCompile(`\bevidence[_-]?[A-Za-z0-9._-]+\b`),
	regexp.MustCompile(`\brun[_-]?[A-Za-z0-9._-]+\b`),
}

var (
	summaryIDLinePattern       = regexp.MustCompile(`(?m)^summary_id:\s*([^\s]+)\s*$`)
	previousSummaryLinePattern = regexp.MustCompile(`(?m)^previous_summary_id:\s*([^\s]+)\s*$`)
	pairRefPattern             = regexp.MustCompile(`\bpair=([A-Za-z0-9._:-]+)`)
)

// CompactionResult is the structured result for a compression pass.
type CompactionResult struct {
	Output  string          `json:"output"`
	Event   CompactionEvent `json:"event"`
	Blocker string          `json:"blocker,omitempty"`
}

// @AX:ANCHOR: [AUTO] @AX:SPEC: SPEC-CONTEXT-COMPRESS-001: compaction event wire schema shared across worker, pipeline, and orchestra
// @AX:REASON: JSON field names and reason/source metadata are emitted through pipeline events and asserted by acceptance tests.
// CompactionEvent records metadata for compaction without raw prompt bodies.
type CompactionEvent struct {
	Type                string   `json:"type"`
	Phase               string   `json:"phase"`
	Provider            string   `json:"provider,omitempty"`
	ModelProfile        string   `json:"model_profile,omitempty"`
	TriggerThreshold    int      `json:"trigger_threshold_tokens"`
	InputEstimate       int      `json:"input_estimate_tokens"`
	OutputEstimate      int      `json:"output_estimate_tokens"`
	SummaryID           string   `json:"summary_id,omitempty"`
	PreviousSummaryID   string   `json:"previous_summary_id,omitempty"`
	PreservedSections   []string `json:"preserved_sections,omitempty"`
	PrunedPairCount     int      `json:"pruned_pair_count"`
	IncompletePairCount int      `json:"incomplete_pair_count,omitempty"`
	ReasonCodes         []string `json:"reason_codes,omitempty"`
	SourceRefs          []string `json:"source_refs,omitempty"`
	CompactionApplied   bool     `json:"compaction_applied"`
}

type pruneDetails struct {
	Text                string
	PrunedPairCount     int
	IncompletePairCount int
	ReasonCodes         []string
}

func newCompactionEvent(phaseName, provider, input string, applied bool) CompactionEvent {
	inputEstimate := EstimateTokens(input)
	return CompactionEvent{
		Type:              EventTypeCompaction,
		Phase:             phaseName,
		Provider:          provider,
		ModelProfile:      provider,
		TriggerThreshold:  WindowSize(provider) / 2,
		InputEstimate:     inputEstimate,
		OutputEstimate:    inputEstimate,
		CompactionApplied: applied,
		ReasonCodes:       []string{ReasonBelowThreshold},
	}
}

func buildCompactionEvent(phaseName, provider, original, summary string, prune pruneDetails) CompactionEvent {
	redactedOriginal, redactionReasons := redactUnsafeContext(original)
	safeOriginal, omittedToolBodies := omitToolPayloadBodies(redactedOriginal)
	redactedPruned, _ := redactUnsafeContext(prune.Text)
	safePruned, omittedPrunedBodies := omitToolPayloadBodies(redactedPruned)
	reasons := []string{ReasonThresholdExceeded}
	reasons = append(reasons, prune.ReasonCodes...)
	if omittedToolBodies || omittedPrunedBodies {
		reasons = append(reasons, ReasonProviderPayloadOmitted)
	}
	for _, reason := range redactionReasons {
		switch reason {
		case "secret":
			reasons = append(reasons, ReasonSecretRedacted)
		case "local_path":
			reasons = append(reasons, ReasonLocalPathRedacted)
		case "provider_payload":
			reasons = append(reasons, ReasonProviderPayloadOmitted)
		}
	}
	if strings.Contains(summary, "context-budget") {
		reasons = append(reasons, ReasonContextBudgetBlocker)
	}
	if strings.Contains(summary, "index_eligibility=redacted_derived_context") {
		reasons = append(reasons, ReasonIndexEligible)
	}
	if strings.Contains(redactedOriginal, "index_export=true") && len(redactionReasons) > 0 {
		reasons = append(reasons, ReasonIndexEligible)
	}

	return CompactionEvent{
		Type:                EventTypeCompaction,
		Phase:               phaseName,
		Provider:            provider,
		ModelProfile:        provider,
		TriggerThreshold:    WindowSize(provider) / 2,
		InputEstimate:       EstimateTokens(original),
		OutputEstimate:      EstimateTokens(summary),
		SummaryID:           extractSummaryID(summary),
		PreviousSummaryID:   extractPreviousSummaryIDFromSummary(summary),
		PreservedSections:   preservedSummarySections(summary),
		PrunedPairCount:     prune.PrunedPairCount,
		IncompletePairCount: prune.IncompletePairCount,
		ReasonCodes:         uniqueStrings(reasons),
		SourceRefs:          extractSourceRefs(safeOriginal + "\n" + safePruned),
		CompactionApplied:   true,
	}
}

func buildSoftPruneEvent(phaseName, provider, original, output string, prune pruneDetails) CompactionEvent {
	redactedOriginal, redactionReasons := redactUnsafeContext(original)
	safeOriginal, omittedOriginalBodies := omitToolPayloadBodies(redactedOriginal)
	redactedOutput, _ := redactUnsafeContext(output)
	safeOutput, omittedOutputBodies := omitToolPayloadBodies(redactedOutput)
	reasons := append([]string{ReasonSoftPrune}, prune.ReasonCodes...)
	if omittedOriginalBodies || omittedOutputBodies {
		reasons = append(reasons, ReasonProviderPayloadOmitted)
	}
	for _, reason := range redactionReasons {
		switch reason {
		case "secret":
			reasons = append(reasons, ReasonSecretRedacted)
		case "local_path":
			reasons = append(reasons, ReasonLocalPathRedacted)
		case "provider_payload":
			reasons = append(reasons, ReasonProviderPayloadOmitted)
		}
	}
	return CompactionEvent{
		Type:                EventTypeCompaction,
		Phase:               phaseName,
		Provider:            provider,
		ModelProfile:        provider,
		TriggerThreshold:    WindowSize(provider) / 2,
		InputEstimate:       EstimateTokens(original),
		OutputEstimate:      EstimateTokens(output),
		PrunedPairCount:     prune.PrunedPairCount,
		IncompletePairCount: prune.IncompletePairCount,
		ReasonCodes:         uniqueStrings(reasons),
		SourceRefs:          extractSourceRefs(safeOriginal + "\n" + safeOutput),
		CompactionApplied:   prune.PrunedPairCount > 0,
	}
}

func extractSummaryID(summary string) string {
	match := summaryIDLinePattern.FindStringSubmatch(summary)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func extractPreviousSummaryIDFromSummary(summary string) string {
	match := previousSummaryLinePattern.FindStringSubmatch(summary)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func preservedSummarySections(summary string) []string {
	sections := []string{
		"Goal",
		"Constraints",
		"Progress",
		"Decisions",
		"Relevant Files",
		"Next Steps",
		"Critical Context",
	}
	var found []string
	for _, section := range sections {
		if strings.Contains(summary, "### "+section+"\n") {
			found = append(found, section)
		}
	}
	return found
}

func extractSourceRefs(text string) []string {
	seen := map[string]bool{}
	var refs []string
	add := func(ref string) {
		if ref == "" || strings.HasPrefix(ref, "/") || seen[ref] {
			return
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	for _, pattern := range sourceRefPatterns {
		for _, ref := range pattern.FindAllString(text, -1) {
			add(ref)
		}
	}
	for _, file := range extractFiles(text) {
		add(file)
	}
	for _, block := range findToolBlocks(text) {
		add(block.pairID)
	}
	for _, match := range pairRefPattern.FindAllStringSubmatch(text, -1) {
		if len(match) == 2 {
			add(match[1])
		}
	}
	return refs
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

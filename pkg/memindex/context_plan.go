package memindex

import (
	"errors"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

// @AX:ANCHOR [AUTO]: Preserve the body-free shadow-plan wire contract and its non-authoritative role.
// @AX:REASON: CLI JSON consumers and regression tests depend on this version while verified v1 delivery remains active.
// @AX:SPEC: SPEC-CONTEXT-ENGINEERING-EVOLUTION-001
const ContextPlanSchemaVersion = "autopus.context_plan.v2"

type ContextPlanOptions struct {
	Delivery              promptlayer.ContextDeliveryResult
	Projection            SearchResponse
	ProjectionError       error
	PinnedReferences      []string
	CandidateBudgetTokens int
	ExpectedReferences    []string
}

type ContextPlanReference struct {
	SourceRef  string `json:"source_ref"`
	SourceHash string `json:"source_hash"`
}

type ContextSelectionHits struct {
	Status        string   `json:"status"`
	ExpectedCount int      `json:"expected_count"`
	HitCount      int      `json:"hit_count"`
	MissCount     int      `json:"miss_count"`
	HitRate       *float64 `json:"hit_rate"`
}

// ContextPlan is a body-free, shadow-only candidate receipt. It never replaces
// the verified v1 delivery used by active dispatch.
type ContextPlan struct {
	SchemaVersion          string                 `json:"schema_version"`
	Status                 string                 `json:"status"`
	ShadowOnly             bool                   `json:"shadow_only"`
	ActiveMode             string                 `json:"active_mode"`
	CandidateMode          string                 `json:"candidate_mode"`
	PinnedReferences       []ContextPlanReference `json:"pinned_references"`
	SelectedReferences     []ContextPlanReference `json:"selected_references"`
	OmittedCount           *int                   `json:"omitted_count"`
	FullTokenEstimate      int                    `json:"full_token_estimate"`
	CandidateTokenEstimate *int                   `json:"candidate_token_estimate"`
	TokenDelta             *int                   `json:"token_delta"`
	ReductionPercent       *float64               `json:"reduction_percent"`
	SelectionHits          ContextSelectionHits   `json:"selection_hits"`
	Reason                 *string                `json:"reason"`
}

// BuildContextPlan computes a deterministic JIT candidate from already
// verified v1 metadata and a memory projection. Search bodies and hashes never
// become authority for the resulting receipt.
// @AX:WARN [AUTO]: This selection policy has more than eight integrity, pinning, freshness, budget, and metric branches.
// @AX:REASON: Reordering or relaxing checks can let shadow projection metadata override verified v1 delivery authority.
func BuildContextPlan(opts ContextPlanOptions) ContextPlan {
	plan := ContextPlan{
		SchemaVersion:     ContextPlanSchemaVersion,
		Status:            "planned",
		ShadowOnly:        true,
		ActiveMode:        "full",
		CandidateMode:     "jit",
		FullTokenEstimate: opts.Delivery.RequiredTokenEstimate,
		SelectionHits: ContextSelectionHits{
			Status: "unavailable",
		},
	}
	if opts.ProjectionError != nil {
		return unavailableContextPlan(plan, projectionReason(opts.ProjectionError))
	}
	if opts.Delivery.SchemaVersion != promptlayer.ContextDeliverySchemaVersion ||
		opts.Delivery.IntegrityStatus != "verified" {
		return unavailableContextPlan(plan, "invalid-delivery")
	}

	documents := make(map[string]promptlayer.ContextDeliveryDocument, len(opts.Delivery.RequiredDocuments))
	for _, document := range opts.Delivery.RequiredDocuments {
		if document.Complete {
			documents[document.SourceRef] = document
		}
	}

	pinnedSet := make(map[string]bool, len(opts.PinnedReferences))
	for _, ref := range cleanReferenceList(opts.PinnedReferences) {
		document, ok := documents[ref]
		if !ok {
			continue
		}
		pinnedSet[ref] = true
		plan.PinnedReferences = append(plan.PinnedReferences, contextPlanReference(document))
	}
	sortContextPlanReferences(plan.PinnedReferences)

	budget := opts.CandidateBudgetTokens
	if budget <= 0 {
		budget = 2_000
	}
	candidateTokens := 0
	for _, ref := range plan.PinnedReferences {
		candidateTokens += documents[ref.SourceRef].TokenEstimate
	}
	selectedSet := make(map[string]bool)
	results := append([]SearchResult(nil), opts.Projection.Results...)
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Rank != results[j].Rank {
			return results[i].Rank < results[j].Rank
		}
		return results[i].SourceRef < results[j].SourceRef
	})
	for _, result := range results {
		document, ok := documents[result.SourceRef]
		if !ok || pinnedSet[result.SourceRef] || selectedSet[result.SourceRef] ||
			strings.HasPrefix(result.SourceRef, strings.TrimSuffix(opts.Delivery.SpecDir, "/")+"/") {
			continue
		}
		if result.FreshnessState != "" && result.FreshnessState != Fresh {
			continue
		}
		if candidateTokens+document.TokenEstimate > budget {
			continue
		}
		selectedSet[result.SourceRef] = true
		candidateTokens += document.TokenEstimate
		plan.SelectedReferences = append(plan.SelectedReferences, contextPlanReference(document))
	}
	sortContextPlanReferences(plan.SelectedReferences)

	omitted := len(opts.Delivery.RequiredDocuments) - len(plan.PinnedReferences) - len(plan.SelectedReferences)
	if omitted < 0 {
		omitted = 0
	}
	delta := plan.FullTokenEstimate - candidateTokens
	if delta < 0 {
		delta = 0
	}
	reduction := 0.0
	if plan.FullTokenEstimate > 0 {
		reduction = float64(delta) * 100 / float64(plan.FullTokenEstimate)
	}
	plan.OmittedCount = intPointer(omitted)
	plan.CandidateTokenEstimate = intPointer(candidateTokens)
	plan.TokenDelta = intPointer(delta)
	plan.ReductionPercent = floatPointer(reduction)
	plan.SelectionHits = buildSelectionHits(opts.ExpectedReferences, pinnedSet, selectedSet)
	return plan
}

func unavailableContextPlan(plan ContextPlan, reason string) ContextPlan {
	plan.Status = "unavailable"
	plan.PinnedReferences = nil
	plan.SelectedReferences = nil
	plan.OmittedCount = nil
	plan.CandidateTokenEstimate = nil
	plan.TokenDelta = nil
	plan.ReductionPercent = nil
	plan.Reason = &reason
	return plan
}

func projectionReason(err error) string {
	var indexed *Error
	if errors.As(err, &indexed) && strings.TrimSpace(indexed.Code) != "" {
		return indexed.Code
	}
	return "projection-unavailable"
}

func contextPlanReference(document promptlayer.ContextDeliveryDocument) ContextPlanReference {
	return ContextPlanReference{SourceRef: document.SourceRef, SourceHash: document.SourceHash}
}

func buildSelectionHits(expected []string, pinned, selected map[string]bool) ContextSelectionHits {
	expected = cleanReferenceList(expected)
	if len(expected) == 0 {
		return ContextSelectionHits{Status: "unavailable"}
	}
	hits := 0
	for _, ref := range expected {
		if pinned[ref] || selected[ref] {
			hits++
		}
	}
	rate := float64(hits) * 100 / float64(len(expected))
	return ContextSelectionHits{
		Status: "available", ExpectedCount: len(expected), HitCount: hits,
		MissCount: len(expected) - hits, HitRate: &rate,
	}
}

func cleanReferenceList(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func sortContextPlanReferences(values []ContextPlanReference) {
	sort.Slice(values, func(i, j int) bool { return values[i].SourceRef < values[j].SourceRef })
}

func intPointer(value int) *int           { return &value }
func floatPointer(value float64) *float64 { return &value }

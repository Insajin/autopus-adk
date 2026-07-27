package memindex_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/memindex"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestBuildContextPlan_FreshProjectionReportsDeterministicShadowMetrics(t *testing.T) {
	t.Parallel()

	delivery := contextPlanDeliveryFixture()
	projection := memindex.SearchResponse{
		SchemaVersion: memindex.SchemaVersion,
		Query:         "raw-query-secret",
		TopK:          3,
		Results: []memindex.SearchResult{
			{
				Rank: 1, SourceRef: "unverified/body.md",
				SourceHash: "sha256:unverified", FreshnessState: memindex.Fresh,
				Summary: "raw provider body must never enter the plan",
			},
			{
				Rank: 2, SourceRef: ".autopus/project/workspace.md",
				SourceHash: "sha256:untrusted-search-hash", FreshnessState: memindex.Fresh,
			},
			{
				Rank: 3, SourceRef: ".autopus/specs/SPEC-PLAN-001/spec.md",
				SourceHash: "sha256:untrusted-search-hash", FreshnessState: memindex.Fresh,
			},
		},
	}

	plan := memindex.BuildContextPlan(memindex.ContextPlanOptions{
		Delivery:              delivery,
		Projection:            projection,
		PinnedReferences:      []string{"AGENTS.md"},
		CandidateBudgetTokens: 2_000,
		ExpectedReferences: []string{
			"AGENTS.md",
			".autopus/project/workspace.md",
			"not-in-v1.md",
		},
	})

	assert.Equal(t, memindex.ContextPlanSchemaVersion, plan.SchemaVersion)
	assert.Equal(t, "planned", plan.Status)
	assert.True(t, plan.ShadowOnly)
	assert.Equal(t, "full", plan.ActiveMode)
	assert.Equal(t, "jit", plan.CandidateMode)
	assert.Equal(t, []memindex.ContextPlanReference{{
		SourceRef: "AGENTS.md", SourceHash: "sha256:agents",
	}}, plan.PinnedReferences)
	assert.Equal(t, []memindex.ContextPlanReference{{
		SourceRef: ".autopus/project/workspace.md", SourceHash: "sha256:workspace",
	}}, plan.SelectedReferences, "selection hashes must come from verified v1 metadata")
	assert.Equal(t, 4, requireInt(t, plan.OmittedCount))
	assert.Equal(t, 10_000, plan.FullTokenEstimate)
	assert.Equal(t, 2_000, requireInt(t, plan.CandidateTokenEstimate))
	assert.Equal(t, 8_000, requireInt(t, plan.TokenDelta))
	assert.Equal(t, 80.0, requireFloat(t, plan.ReductionPercent))
	assert.Equal(t, "available", plan.SelectionHits.Status)
	assert.Equal(t, 2, plan.SelectionHits.HitCount)
	assert.Equal(t, 3, plan.SelectionHits.ExpectedCount)
	assert.InDelta(t, 66.6667, requireFloat(t, plan.SelectionHits.HitRate), 0.0001)

	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	assertContextPlanJSONKeys(t, encoded)
	for _, forbidden := range []string{
		projection.Query,
		projection.Results[0].Summary,
		"unverified/body.md",
		"sha256:untrusted-search-hash",
		"/Users/example/project",
	} {
		assert.NotContains(t, string(encoded), forbidden)
	}
}

func TestBuildContextPlan_NoSelectionLabelsKeepsArithmeticAndNullHitRate(t *testing.T) {
	t.Parallel()

	plan := memindex.BuildContextPlan(memindex.ContextPlanOptions{
		Delivery: contextPlanDeliveryFixture(),
		Projection: memindex.SearchResponse{
			SchemaVersion: memindex.SchemaVersion,
			Results: []memindex.SearchResult{{
				SourceRef: ".autopus/project/workspace.md", FreshnessState: memindex.Fresh,
			}},
		},
		PinnedReferences:      []string{"AGENTS.md"},
		CandidateBudgetTokens: 2_000,
	})

	assert.Equal(t, "planned", plan.Status)
	assert.Equal(t, 8_000, requireInt(t, plan.TokenDelta))
	assert.Equal(t, 80.0, requireFloat(t, plan.ReductionPercent))
	assert.Equal(t, "unavailable", plan.SelectionHits.Status)
	assert.Zero(t, plan.SelectionHits.HitCount)
	assert.Zero(t, plan.SelectionHits.ExpectedCount)
	assert.Nil(t, plan.SelectionHits.HitRate)

	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &raw))
	var selectionHits map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["selection_hits"], &selectionHits))
	assert.Equal(t, "null", string(selectionHits["hit_rate"]))
}

func TestBuildContextPlan_ProjectionFailuresAreBodyFreeAndNonGating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
		err  error
	}{
		{
			name: "missing", code: "rebuild-required",
			err: &memindex.Error{
				Code: "rebuild-required",
				Err:  errors.New("missing /Users/example/project/.autopus/runtime/index"),
			},
		},
		{
			name: "stale", code: "stale-source",
			err: &memindex.Error{
				Code: "stale-source",
				Err:  errors.New("stale prompt body sk-proj-secret"),
			},
		},
		{
			name: "corrupt", code: "projection-corrupt",
			err: &memindex.Error{
				Code: "projection-corrupt",
				Err:  errors.New("provider payload and raw query leaked"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan := memindex.BuildContextPlan(memindex.ContextPlanOptions{
				Delivery:        contextPlanDeliveryFixture(),
				ProjectionError: tt.err,
			})

			assert.Equal(t, "unavailable", plan.Status)
			assert.Equal(t, 10_000, plan.FullTokenEstimate)
			assert.Nil(t, plan.PinnedReferences)
			assert.Nil(t, plan.SelectedReferences)
			assert.Nil(t, plan.OmittedCount)
			assert.Nil(t, plan.CandidateTokenEstimate)
			assert.Nil(t, plan.TokenDelta)
			assert.Nil(t, plan.ReductionPercent)
			assert.Equal(t, "unavailable", plan.SelectionHits.Status)
			assert.Nil(t, plan.SelectionHits.HitRate)
			assert.Equal(t, tt.code, requireString(t, plan.Reason))

			encoded, err := json.Marshal(plan)
			require.NoError(t, err)
			assertContextPlanJSONKeys(t, encoded)
			for _, forbidden := range []string{
				"/Users/example/project",
				"sk-proj-secret",
				"provider payload",
				"raw query",
			} {
				assert.NotContains(t, string(encoded), forbidden)
			}
		})
	}
}

func contextPlanDeliveryFixture() promptlayer.ContextDeliveryResult {
	documents := []promptlayer.ContextDeliveryDocument{
		{SourceRef: "AGENTS.md", SourceHash: "sha256:agents", TokenEstimate: 1_000, Complete: true},
		{SourceRef: ".autopus/project/workspace.md", SourceHash: "sha256:workspace", TokenEstimate: 1_000, Complete: true},
		{SourceRef: "ARCHITECTURE.md", SourceHash: "sha256:architecture", TokenEstimate: 1_000, Complete: true},
		{SourceRef: ".autopus/specs/SPEC-PLAN-001/spec.md", SourceHash: "sha256:spec", TokenEstimate: 1_000, Complete: true},
		{SourceRef: ".autopus/specs/SPEC-PLAN-001/plan.md", SourceHash: "sha256:plan", TokenEstimate: 3_000, Complete: true},
		{SourceRef: ".autopus/specs/SPEC-PLAN-001/acceptance.md", SourceHash: "sha256:acceptance", TokenEstimate: 3_000, Complete: true},
	}
	return promptlayer.ContextDeliveryResult{
		SchemaVersion:         promptlayer.ContextDeliverySchemaVersion,
		Command:               "go",
		SpecDir:               ".autopus/specs/SPEC-PLAN-001",
		RequiredDocuments:     documents,
		RequiredTokenEstimate: 10_000,
		IntegrityStatus:       "verified",
	}
}

func assertContextPlanJSONKeys(t *testing.T, encoded []byte) {
	t.Helper()
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &raw))
	assert.ElementsMatch(t, []string{
		"schema_version",
		"status",
		"shadow_only",
		"active_mode",
		"candidate_mode",
		"pinned_references",
		"selected_references",
		"omitted_count",
		"full_token_estimate",
		"candidate_token_estimate",
		"token_delta",
		"reduction_percent",
		"selection_hits",
		"reason",
	}, mapKeys(raw))
}

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func requireInt(t *testing.T, value *int) int {
	t.Helper()
	require.NotNil(t, value)
	return *value
}

func requireFloat(t *testing.T, value *float64) float64 {
	t.Helper()
	require.NotNil(t, value)
	return *value
}

func requireString(t *testing.T, value *string) string {
	t.Helper()
	require.NotNil(t, value)
	return *value
}

package cli

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSpecReviewLoop_KeepsProviderFindingsWhenReviseConsensusIsUnique(t *testing.T) {
	tests := []struct {
		name      string
		responses []orchestra.ProviderResponse
		wantMin   int
	}{
		{
			name: "debate outputs with idless findings and finding statuses",
			responses: []orchestra.ProviderResponse{
				{
					Provider: "claude",
					Output: `{"verdict":"REVISE","summary":"three blockers","findings":[` +
						`{"severity":"major","category":"completeness","scope_ref":"plan.md:12","location":"plan.md:12","description":"Missing revision closure criteria.","suggestion":"Add closure criteria."},` +
						`{"severity":"major","category":"correctness","scope_ref":"acceptance.md:20","location":"acceptance.md:20","description":"Acceptance does not cover malformed redline input.","suggestion":"Add malformed-input scenario."},` +
						`{"severity":"minor","category":"feasibility","scope_ref":"research.md:44","location":"research.md:44","description":"Rollback evidence is underspecified.","suggestion":"Name the rollback artifact."}` +
						`],"finding_statuses":[{"id":"F1","status":"open","reason":"still open"},{"id":"F2","status":"open","reason":"still open"},{"id":"F3","status":"open","reason":"still open"}]}`,
				},
				{
					Provider: "codex",
					Output: `{"verdict":"REVISE","summary":"four blockers","findings":[` +
						`{"severity":"major","category":"completeness","scope_ref":"spec.md:33","location":"spec.md:33","description":"Outcome lock omits review persistence behavior.","suggestion":"State persistence behavior."},` +
						`{"severity":"major","category":"correctness","scope_ref":"plan.md:50","location":"plan.md:50","description":"Plan does not mention parser failure recovery.","suggestion":"Add recovery step."},` +
						`{"severity":"minor","category":"style","scope_ref":"research.md:80","location":"research.md:80","description":"Reviewer brief mixes evidence and speculation.","suggestion":"Separate evidence from speculation."},` +
						`{"severity":"major","category":"feasibility","scope_ref":"acceptance.md:72","location":"acceptance.md:72","description":"No acceptance case for concurrent edit conflict.","suggestion":"Add concurrent conflict scenario."}` +
						`],"finding_statuses":[]}`,
				},
				{
					Provider: "gemini",
					Output:   `{"verdict":"PASS","summary":"ok","findings":[]}`,
				},
			},
			wantMin: 7,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			specID := "SPEC-REVIEW-FINDINGS-KEEP-001"
			specDir := scaffoldReviewSpec(t, dir, specID)
			doc, err := spec.Load(specDir)
			require.NoError(t, err)

			origRunner := specReviewRunOrchestra
			specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
				return &orchestra.OrchestraResult{Responses: tt.responses}, nil
			}
			defer func() { specReviewRunOrchestra = origRunner }()

			params := reviewLoopParams(specID, specDir)
			params.maxRevisions = 0
			result, err := runSpecReviewLoop(params, doc, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, spec.VerdictRevise, result.Verdict)
			require.GreaterOrEqual(t, len(result.Findings), tt.wantMin)

			persisted, err := spec.LoadFindings(specDir)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(persisted), tt.wantMin)
		})
	}
}

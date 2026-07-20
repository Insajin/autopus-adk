package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeResponse(provider, output string) ProviderResponse {
	return ProviderResponse{
		Provider: provider,
		Output:   output,
		Duration: 100 * time.Millisecond,
		ExitCode: 0,
	}
}

func TestGetStrategyFunc_Valid(t *testing.T) {
	t.Parallel()
	for _, s := range ValidStrategies {
		s := s
		t.Run(string(s), func(t *testing.T) {
			t.Parallel()
			fn, err := GetStrategyFunc(s)
			require.NoError(t, err)
			assert.NotNil(t, fn)
		})
	}
}

func TestGetStrategyFunc_Invalid(t *testing.T) {
	t.Parallel()
	_, err := GetStrategyFunc(Strategy("unknown"))
	assert.Error(t, err)
}

func TestHandleConsensus_ThreeResponses(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		makeResponse("p1", "golang is great\npython is popular"),
		makeResponse("p2", "golang is great\nrust is fast"),
		makeResponse("p3", "golang is great\njavascript is everywhere"),
	}

	cfg := OrchestraConfig{Strategy: StrategyConsensus}
	merged, summary, err := handleConsensus(context.Background(), responses, cfg)
	require.NoError(t, err)
	// "golang is great"는 3/3이므로 합의 항목
	assert.Contains(t, merged, "golang is great")
	assert.Contains(t, summary, "합의율")
}

func TestHandleConsensus_NoResponses(t *testing.T) {
	t.Parallel()
	merged, summary, err := handleConsensus(context.Background(), nil, OrchestraConfig{})
	require.NoError(t, err)
	assert.Empty(t, merged)
	assert.Contains(t, summary, "응답 없음")
}

func TestHandlePipeline_MultipleStages(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		makeResponse("designer", "## Design\nuse hexagonal architecture"),
		makeResponse("coder", "## Implementation\npackage main"),
		makeResponse("reviewer", "## Review\nlooks good"),
	}

	cfg := OrchestraConfig{Strategy: StrategyPipeline}
	merged, summary, err := handlePipeline(context.Background(), responses, cfg)
	require.NoError(t, err)
	assert.Contains(t, merged, "Stage 1")
	assert.Contains(t, merged, "Stage 2")
	assert.Contains(t, merged, "Stage 3")
	assert.Contains(t, summary, "3단계")
}

func TestHandleDebate_TwoResponses(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		makeResponse("claude", "use microservices\ndecouple everything"),
		makeResponse("codex", "use monolith\ndecouple everything"),
	}

	cfg := OrchestraConfig{Strategy: StrategyDebate, JudgeProvider: "gemini"}
	merged, summary, err := handleDebate(context.Background(), responses, cfg)
	require.NoError(t, err)
	assert.Contains(t, merged, "claude의 의견")
	assert.Contains(t, merged, "codex의 의견")
	assert.Contains(t, summary, "판정 실패: gemini")
}

func TestHandleDebate_NoJudge(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		makeResponse("p1", "option a"),
		makeResponse("p2", "option b"),
	}

	cfg := OrchestraConfig{Strategy: StrategyDebate, JudgeProvider: ""}
	_, summary, err := handleDebate(context.Background(), responses, cfg)
	require.NoError(t, err)
	assert.Contains(t, summary, "판정: 없음")
}

func TestHandleFastest_SingleResponse(t *testing.T) {
	t.Parallel()
	responses := []ProviderResponse{
		makeResponse("speedster", "quick answer"),
	}

	cfg := OrchestraConfig{Strategy: StrategyFastest}
	merged, summary, err := handleFastest(context.Background(), responses, cfg)
	require.NoError(t, err)
	assert.Equal(t, "quick answer", merged)
	assert.Contains(t, summary, "최속 응답: speedster")
}

func TestHandleFastest_NoResponses(t *testing.T) {
	t.Parallel()
	_, _, err := handleFastest(context.Background(), nil, OrchestraConfig{})
	assert.Error(t, err)
}

func TestHandleConsensus_BelowThreshold(t *testing.T) {
	t.Parallel()
	// 각 응답이 완전히 다른 내용을 가질 때 합의율이 낮아야 한다
	responses := []ProviderResponse{
		makeResponse("p1", "golang"),
		makeResponse("p2", "python"),
		makeResponse("p3", "rust"),
	}

	cfg := OrchestraConfig{Strategy: StrategyConsensus}
	merged, summary, err := handleConsensus(context.Background(), responses, cfg)
	require.NoError(t, err)
	// 합의된 항목이 없으므로 이견 섹션이 있어야 한다
	assert.Contains(t, merged, "이견")
	assert.Contains(t, summary, "합의율")
	_ = merged
}

// --- SPEC-ORCHCFG-001 Phase 1.5: Threshold Test Scaffolds ---

// R3: handleConsensus must use OrchestraConfig.ConsensusThreshold instead of hardcoded 0.66.
// This test sets threshold=1.0 so only unanimous lines pass, then verifies that
// a line appearing in 2/3 responses is classified as dissent rather than consensus.
func TestHandleConsensus_UsesConfigThreshold(t *testing.T) {
	t.Parallel()

	// "python is popular" appears in 2 of 3 responses (67%). With
	// threshold=1.0 it must remain visible outside the consensus section.
	responses := []ProviderResponse{
		makeResponse("p1", "golang is great\npython is popular"),
		makeResponse("p2", "golang is great\npython is popular"),
		makeResponse("p3", "golang is great\nrust is fast"),
	}

	cfg := OrchestraConfig{
		Strategy:           StrategyConsensus,
		ConsensusThreshold: 1.0, // require 100% agreement
	}
	merged, _, err := handleConsensus(context.Background(), responses, cfg)
	require.NoError(t, err)
	assert.Contains(t, merged, "✓ golang is great")
	assert.Contains(t, merged, "## 이견이 있는 내용")
	assert.Contains(t, merged, "△ python is popular [p1, p2]",
		"handleConsensus must preserve 67% evidence as dissent at threshold=1.0")
	assert.Contains(t, merged, "△ rust is fast [p3]")
}

// R3: handleConsensus with zero threshold should use default 0.66.
// This test sets threshold=0 and checks a line at exactly 33% (1/3) is excluded.
// Currently handleConsensus hardcodes 0.66 so this also excludes it — making the
// test pass. We need a complementary assertion: set threshold=0.2 and verify the
// same line IS included (which the hardcoded 0.66 will reject).
func TestHandleConsensus_LowThresholdIncludesMoreLines(t *testing.T) {
	t.Parallel()

	// "rust is fast" appears in 1 of 3 responses (33%).
	// With threshold=0.2 it should appear in consensus.
	// With the hardcoded 0.66 it will NOT — proving the config is ignored.
	responses := []ProviderResponse{
		makeResponse("p1", "golang is great\npython is popular"),
		makeResponse("p2", "golang is great\npython is popular"),
		makeResponse("p3", "golang is great\nrust is fast"),
	}

	cfg := OrchestraConfig{
		Strategy:           StrategyConsensus,
		ConsensusThreshold: 0.2, // low threshold — 33% should pass
	}
	merged, _, err := handleConsensus(context.Background(), responses, cfg)
	require.NoError(t, err)
	// "rust is fast" is 1/3 (33%) — with threshold=0.2 it must appear in the
	// consensus section (marked with checkmark), NOT in the disagreement section.
	// The hardcoded 0.66 will put it in disagreement instead.
	assert.Contains(t, merged, "\u2713 rust is fast",
		"handleConsensus should respect ConsensusThreshold=0.2 and include 33% lines in consensus")
}

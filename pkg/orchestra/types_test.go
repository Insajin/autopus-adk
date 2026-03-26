package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrategy_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		strategy Strategy
		want     bool
	}{
		{"consensus 유효", StrategyConsensus, true},
		{"pipeline 유효", StrategyPipeline, true},
		{"debate 유효", StrategyDebate, true},
		{"fastest 유효", StrategyFastest, true},
		{"빈 문자열 무효", Strategy(""), false},
		{"알 수 없는 전략 무효", Strategy("unknown"), false},
		{"대소문자 구분", Strategy("Consensus"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.strategy.IsValid())
		})
	}
}

func TestValidStrategies_Count(t *testing.T) {
	t.Parallel()
	assert.Len(t, ValidStrategies, 5)
}

func TestOrchestraConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "claude", Args: []string{"--print"}},
		},
		Strategy:       StrategyConsensus,
		Prompt:         "테스트 프롬프트",
		TimeoutSeconds: 30,
		JudgeProvider:  "claude",
	}
	assert.Equal(t, StrategyConsensus, cfg.Strategy)
	assert.Equal(t, 30, cfg.TimeoutSeconds)
	assert.Equal(t, "claude", cfg.JudgeProvider)
}

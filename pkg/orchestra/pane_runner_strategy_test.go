package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeByStrategy_ConsensusUsesConfiguredThreshold(t *testing.T) {
	t.Parallel()

	responses := []ProviderResponse{
		{Provider: "p1", Output: "1. shared\n2. pair"},
		{Provider: "p2", Output: "1. shared\n2. pair"},
		{Provider: "p3", Output: "1. shared"},
	}
	cfg := OrchestraConfig{ConsensusThreshold: 1.0}

	merged, _ := mergeByStrategy(StrategyConsensus, responses, cfg)

	assert.Contains(t, merged, "✓ 1. shared")
	assert.Contains(t, merged, "## 이견이 있는 내용")
	assert.Contains(t, merged, "△ 2. pair [2/3]")
}

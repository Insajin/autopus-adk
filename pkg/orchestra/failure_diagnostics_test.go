package orchestra

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOrchestra_AllProvidersFail_ReturnsFailureResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	result, err := RunOrchestra(context.Background(), OrchestraConfig{
		Providers: []ProviderConfig{
			emptyOutputProvider("claude"),
			emptyOutputProvider("codex"),
			badArgsProvider("gemini"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "diagnose orchestra failures",
		TimeoutSeconds: 10,
	})

	require.Error(t, err)
	require.NotNil(t, result)
	require.Len(t, result.FailedProviders, 3)
	assert.Empty(t, result.Responses)
	assert.Contains(t, result.Summary, "all providers failed")
	assert.Contains(t, err.Error(), "모든 프로바이더가 실패했습니다")
}

func TestBuildFailedProvider_ClassifiesTimeoutAndCapacity(t *testing.T) {
	t.Parallel()

	timeoutFailure := buildFailedProvider(
		ProviderConfig{Name: "claude", StartupTimeout: 30 * time.Millisecond},
		&ProviderResponse{Provider: "claude", TimedOut: true},
		nil,
		120,
	)
	capacityFailure := buildFailedProvider(
		ProviderConfig{Name: "gemini"},
		&ProviderResponse{Provider: "gemini", Error: "status: RESOURCE_EXHAUSTED reason: MODEL_CAPACITY_EXHAUSTED"},
		assert.AnError,
		120,
	)

	assert.Equal(t, "timeout", timeoutFailure.FailureClass)
	assert.Contains(t, timeoutFailure.Error, "deadline")
	assert.Contains(t, timeoutFailure.NextRemediation, "increase timeout")

	assert.Equal(t, "capacity_exhausted", capacityFailure.FailureClass)
	assert.Contains(t, capacityFailure.NextRemediation, "retry later")
}

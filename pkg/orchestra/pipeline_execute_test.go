package orchestra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type diagnosticBackend struct{}

func (diagnosticBackend) Execute(_ context.Context, req ProviderRequest) (*ProviderResponse, error) {
	switch req.Provider {
	case "claude":
		return &ProviderResponse{Provider: "claude", Output: `{"analysis":"ok"}`}, nil
	case "gemini":
		return &ProviderResponse{
			Provider: "gemini",
			Output:   "partial stdout before failure",
			Error:    "RESOURCE_EXHAUSTED MODEL_CAPACITY_EXHAUSTED: no capacity available",
		}, fmt.Errorf("subprocess gemini exited with code 1")
	default:
		return nil, fmt.Errorf("unexpected provider %s", req.Provider)
	}
}

func (diagnosticBackend) Name() string {
	return "diagnostic"
}

func TestExecuteParallel_UsesStructuredFailedProviderDiagnostics(t *testing.T) {
	t.Parallel()

	providers := []ProviderConfig{
		{Name: "claude", Binary: "claude"},
		{Name: "gemini", Binary: "gemini", ExecutionTimeout: 45 * time.Second},
	}

	successes, failed, err := executeParallel(
		context.Background(),
		diagnosticBackend{},
		providers,
		"prompt",
		"",
		"debater_r1",
		1,
		120,
	)

	require.NoError(t, err)
	require.Len(t, successes, 1)
	assert.Equal(t, "claude", successes[0].Provider)
	require.Len(t, failed, 1)
	assert.Equal(t, "gemini", failed[0].Name)
	assert.Equal(t, "capacity_exhausted", failed[0].FailureClass)
	assert.Contains(t, failed[0].Error, "subprocess gemini exited")
	assert.Equal(t, "retry later or reduce provider set", failed[0].NextRemediation)
	assert.Contains(t, failed[0].StderrPreview, "RESOURCE_EXHAUSTED MODEL_CAPACITY_EXHAUSTED")
	assert.Contains(t, failed[0].OutputPreview, "partial stdout")
}

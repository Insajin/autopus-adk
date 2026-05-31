package cli

import (
	"context"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStructuredSpecReviewOrchestra_PreservesPaneFailureMode(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		name: "pane",
		outputs: map[string]orchestra.ProviderResponse{
			"claude": {
				EmptyOutput:     true,
				ExecutedBackend: "pane",
				Duration:        2 * time.Second,
			},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers:      []orchestra.ProviderConfig{{Name: "claude", Binary: "claude"}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
	})
	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	require.Len(t, result.FailedProviders, 1)
	assert.Equal(t, "pane", result.Responses[0].ExecutedBackend)
	assert.Equal(t, "pane", result.FailedProviders[0].CollectionMode)
}

package cli

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStructuredSpecReviewOrchestra_InlinesSchemaForPaneBackendEvenWithSchemaFlag(t *testing.T) {
	backend := &fakeStructuredReviewBackend{
		name: "pane",
		outputs: map[string]orchestra.ProviderResponse{
			"codex": {Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
		},
	}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers:      []orchestra.ProviderConfig{{Name: "codex", Binary: "codex", SchemaFlag: "--output-schema"}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
	})
	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	assert.Empty(t, result.FailedProviders)

	backend.mu.Lock()
	defer backend.mu.Unlock()
	require.Len(t, backend.requests, 1)
	assert.Contains(t, backend.requests[0].Prompt, "Required JSON schema",
		"pane backend cannot pass provider schema flags, so the schema must be inlined")
}

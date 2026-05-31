package cli

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/stretchr/testify/assert"
)

func TestShouldTreatOrchestraResultAsFailure_AllResponsesFailed(t *testing.T) {
	t.Parallel()

	result := &orchestra.OrchestraResult{
		Responses: []orchestra.ProviderResponse{{
			Provider: "gemini",
			TimedOut: true,
			Output:   "shell quote>",
		}},
		FailedProviders: []orchestra.FailedProvider{{
			Name:         "gemini",
			FailureClass: "timeout",
			Error:        "provider timed out",
		}},
	}

	assert.True(t, shouldTreatOrchestraResultAsFailure(result))
}

func TestShouldTreatOrchestraResultAsFailure_PartialSuccess(t *testing.T) {
	t.Parallel()

	result := &orchestra.OrchestraResult{
		Responses: []orchestra.ProviderResponse{
			{Provider: "gemini", TimedOut: true},
			{Provider: "claude", Output: "ok"},
		},
		FailedProviders: []orchestra.FailedProvider{{Name: "gemini"}},
	}

	assert.False(t, shouldTreatOrchestraResultAsFailure(result))
}

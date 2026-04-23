package orchestra

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProviderExecutionTimeout_PrefersExecutionTimeoutOverStartupTimeout(t *testing.T) {
	t.Parallel()

	provider := ProviderConfig{
		Name:             "gemini",
		StartupTimeout:   20 * time.Second,
		ExecutionTimeout: 180 * time.Second,
	}

	assert.Equal(t, 180*time.Second, providerExecutionTimeout(provider, 120))
}

func TestProviderExecutionTimeout_FallsBackToCommandTimeout(t *testing.T) {
	t.Parallel()

	provider := ProviderConfig{
		Name:           "gemini",
		StartupTimeout: 20 * time.Second,
	}

	assert.Equal(t, 120*time.Second, providerExecutionTimeout(provider, 120))
}

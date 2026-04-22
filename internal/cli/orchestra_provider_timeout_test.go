package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestResolveProviders_DefaultStartupTimeoutPropagated(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{
			"gemini": {Binary: "gemini", Args: []string{"-m", "gemini-3.1-pro-preview", "-p", ""}, PromptViaArgs: false},
		},
		Commands: map[string]config.CommandEntry{},
	}

	providers := resolveProviders(conf, "review", []string{"gemini"})
	require.Len(t, providers, 1)
	assert.Equal(t, defaultProviderStartupTimeout("gemini"), providers[0].StartupTimeout)
}

func TestResolveProviders_SubprocessTimeoutOverridesDefaultStartupTimeout(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{
			"gemini": {
				Binary:        "gemini",
				Args:          []string{"-m", "gemini-3.1-pro-preview", "-p", ""},
				PromptViaArgs: false,
				Subprocess:    config.SubprocessProvConf{Timeout: 7},
			},
		},
		Commands: map[string]config.CommandEntry{},
	}

	providers := resolveProviders(conf, "review", []string{"gemini"})
	require.Len(t, providers, 1)
	assert.Equal(t, 7*time.Second, providers[0].StartupTimeout)
}

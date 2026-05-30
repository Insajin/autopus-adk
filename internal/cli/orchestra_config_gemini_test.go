package cli

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveProviders_GeminiInteractiveInputPreventsArgsDerivation(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{
			"gemini": {
				Binary:           "agy",
				Args:             []string{"--print", ""},
				PaneArgs:         []string{},
				PromptViaArgs:    true,
				InteractiveInput: "stdin",
			},
		},
		Commands: map[string]config.CommandEntry{},
	}

	providers := resolveProviders(conf, "review", []string{"gemini"})
	require.Len(t, providers, 1)
	assert.Equal(t, "stdin", providers[0].InteractiveInput)
	assert.NotEqual(t, "args", providers[0].InteractiveInput)
}

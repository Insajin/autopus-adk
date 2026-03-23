package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestResolveProviders_FlagOverride(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{
			"claude": {Binary: "claude", Args: []string{"--print"}},
			"gemini": {Binary: "gemini", Args: []string{}, PromptViaArgs: true},
		},
		Commands: map[string]config.CommandEntry{
			"review": {Strategy: "debate", Providers: []string{"claude", "gemini"}},
		},
	}

	// CLI flags override config
	providers := resolveProviders(conf, "review", []string{"claude"})
	require.Len(t, providers, 1)
	assert.Equal(t, "claude", providers[0].Name)
}

func TestResolveProviders_CommandConfig(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{
			"claude": {Binary: "claude", Args: []string{"--print"}},
			"gemini": {Binary: "gemini", Args: []string{}, PromptViaArgs: true},
			"codex":  {Binary: "codex", Args: []string{"--quiet"}},
		},
		Commands: map[string]config.CommandEntry{
			"review": {Strategy: "debate", Providers: []string{"claude", "gemini"}},
		},
	}

	// No flag providers: fall back to command config
	providers := resolveProviders(conf, "review", nil)
	require.Len(t, providers, 2)

	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name
	}
	assert.Contains(t, names, "claude")
	assert.Contains(t, names, "gemini")
}

func TestResolveProviders_AllConfigProviders(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{
			"claude": {Binary: "claude", Args: []string{"--print"}},
			"gemini": {Binary: "gemini", Args: []string{}, PromptViaArgs: true},
		},
		Commands: map[string]config.CommandEntry{},
	}

	// No flags, no command config: fall back to all providers
	providers := resolveProviders(conf, "review", nil)
	assert.Len(t, providers, 2)
}

func TestResolveProviders_PromptViaArgsPropagated(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{
			"gemini": {Binary: "gemini", Args: []string{}, PromptViaArgs: true},
			"claude": {Binary: "claude", Args: []string{"--print"}, PromptViaArgs: false},
		},
		Commands: map[string]config.CommandEntry{},
	}

	providers := resolveProviders(conf, "review", []string{"gemini", "claude"})
	require.Len(t, providers, 2)

	for _, p := range providers {
		if p.Name == "gemini" {
			assert.True(t, p.PromptViaArgs, "gemini must have PromptViaArgs=true")
		}
		if p.Name == "claude" {
			assert.False(t, p.PromptViaArgs, "claude must have PromptViaArgs=false")
		}
	}
}

func TestResolveProviders_UnknownProviderFallback(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{},
		Commands:  map[string]config.CommandEntry{},
	}

	// Unknown provider name: fallback to binary=name, args=[], PromptViaArgs=false
	providers := resolveProviders(conf, "review", []string{"unknown-tool"})
	require.Len(t, providers, 1)
	assert.Equal(t, "unknown-tool", providers[0].Name)
	assert.Equal(t, "unknown-tool", providers[0].Binary)
	assert.False(t, providers[0].PromptViaArgs)
}

func TestResolveStrategy_FlagOverride(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		DefaultStrategy: "consensus",
		Commands: map[string]config.CommandEntry{
			"review": {Strategy: "debate"},
		},
	}

	// Flag overrides config
	s := resolveStrategy(conf, "review", "fastest")
	assert.Equal(t, "fastest", s)
}

func TestResolveStrategy_CommandConfig(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		DefaultStrategy: "consensus",
		Commands: map[string]config.CommandEntry{
			"review": {Strategy: "debate"},
		},
	}

	// No flag: command config used
	s := resolveStrategy(conf, "review", "")
	assert.Equal(t, "debate", s)
}

func TestResolveStrategy_DefaultStrategy(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		DefaultStrategy: "consensus",
		Commands:        map[string]config.CommandEntry{},
	}

	// No flag, no command config: default strategy
	s := resolveStrategy(conf, "plan", "")
	assert.Equal(t, "consensus", s)
}

func TestResolveStrategy_FallbackConsensus(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		DefaultStrategy: "",
		Commands:        map[string]config.CommandEntry{},
	}

	// No config at all: hardcoded "consensus"
	s := resolveStrategy(conf, "plan", "")
	assert.Equal(t, "consensus", s)
}

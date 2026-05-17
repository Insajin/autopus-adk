package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultProviderEntries_ClaudeEffortHigh(t *testing.T) {
	t.Parallel()

	claude, ok := defaultProviderEntries["claude"]
	require.True(t, ok, "claude must exist in defaultProviderEntries")

	assert.Contains(t, claude.Args, "high")
	assert.NotContains(t, claude.Args, "max")
	assert.Contains(t, claude.PaneArgs, "high")
	assert.NotContains(t, claude.PaneArgs, "max")
}

func TestMigrateOrchestraConfig_ClaudeDeprecatedMaxEffort(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code"},
		Orchestra: OrchestraConf{
			Enabled: true,
			Providers: map[string]ProviderEntry{
				"claude": {
					Binary:   "claude",
					Args:     []string{"--print", "--model", "opus", "--effort", "max"},
					PaneArgs: []string{"-p", "--model", "opus", "--effort", "max"},
				},
			},
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.True(t, changed)

	claude := cfg.Orchestra.Providers["claude"]
	assert.Equal(t, []string{"--print", "--model", "opus", "--effort", "high"}, claude.Args)
	assert.Equal(t, []string{"-p", "--model", "opus", "--effort", "high"}, claude.PaneArgs)
}

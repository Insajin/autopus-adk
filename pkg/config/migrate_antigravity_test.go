package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateOrchestraConfig_AntigravityMigratesLegacyGeminiProvider(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"antigravity-cli"},
		Orchestra: OrchestraConf{
			Enabled: true,
			Providers: map[string]ProviderEntry{
				"gemini": {Binary: "gemini", Args: []string{"-m", "gemini-3.1-pro-preview", "-p", ""}},
			},
		},
	}

	changed, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "agy", cfg.Orchestra.Providers["gemini"].Binary)
	// SPEC-ORCH-021 REQ-014: prompt is the value of --print (filled into "" slot).
	assert.Equal(t, []string{"--print", ""}, cfg.Orchestra.Providers["gemini"].Args)
}

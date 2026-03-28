package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- R3: opencode PromptViaArgs=false ---

// TestMigrateOpencodeToTUI_SetsPromptViaArgsFalse verifies that MigrateOpencodeToTUI
// sets PromptViaArgs=false for opencode provider.
func TestMigrateOpencodeToTUI_SetsPromptViaArgsFalse(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Orchestra: OrchestraConf{
			Enabled: true,
			Providers: map[string]ProviderEntry{
				"opencode": {
					Binary:           "opencode",
					Args:             []string{"run", "-m", "openai/gpt-5.4"},
					PaneArgs:         []string{"-m", "openai/gpt-5.4"},
					PromptViaArgs:    true,
					InteractiveInput: "args",
				},
			},
		},
	}

	migrated := MigrateOpencodeToTUI(cfg)
	assert.True(t, migrated, "migration must be applied")

	oc := cfg.Orchestra.Providers["opencode"]
	assert.False(t, oc.PromptViaArgs,
		"MigrateOpencodeToTUI must set PromptViaArgs=false (R3)")
	assert.Empty(t, oc.InteractiveInput,
		"MigrateOpencodeToTUI must clear InteractiveInput (R3)")
}

// TestDefaultProviderEntries_OpencodePromptViaArgsFalse verifies the canonical
// default entry for opencode has PromptViaArgs=false.
func TestDefaultProviderEntries_OpencodePromptViaArgsFalse(t *testing.T) {
	t.Parallel()

	entry, ok := defaultProviderEntries["opencode"]
	require.True(t, ok, "opencode must exist in defaultProviderEntries")
	assert.False(t, entry.PromptViaArgs,
		"opencode default must have PromptViaArgs=false (R3)")
}

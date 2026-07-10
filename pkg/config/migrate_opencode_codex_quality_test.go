package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateOpencodeToCodex_EmptyExistingCodexUsesOwnershipSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing ProviderEntry
		want     ProviderEntry
	}{
		{
			name:     "quality managed resets to quality default",
			existing: ProviderEntry{Binary: "stale-codex", PaneArgs: []string{"--stale"}, ModelPolicy: ProviderModelPolicyQuality},
		},
		{name: "zero-value unmarked resets to quality default"},
		{
			name:     "unmarked custom signals remain user owned",
			existing: ProviderEntry{Binary: "codex-wrapper", PaneArgs: []string{"--custom-pane"}, PromptViaArgs: true},
			want:     ProviderEntry{Binary: "codex-wrapper", PaneArgs: []string{"--custom-pane"}, ModelPolicy: ProviderModelPolicyPinned, PromptViaArgs: true},
		},
		{
			name:     "explicit pinned remains user owned",
			existing: ProviderEntry{Binary: "codex-wrapper", PaneArgs: []string{"--custom-pane"}, ModelPolicy: ProviderModelPolicyPinned},
			want:     ProviderEntry{Binary: "codex-wrapper", PaneArgs: []string{"--custom-pane"}, ModelPolicy: ProviderModelPolicyPinned},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &HarnessConfig{
				Quality: QualityConf{Default: "ultra"},
				Orchestra: OrchestraConf{
					Enabled: true,
					Providers: map[string]ProviderEntry{
						"codex":    tt.existing,
						"opencode": {Binary: "opencode"},
					},
				},
			}
			want := tt.want
			if want.Binary == "" {
				want = CodexProviderEntryForQuality(cfg.Quality)
			}

			changed, err := MigrateOpencodeToCodex(cfg)
			require.NoError(t, err)
			assert.True(t, changed)
			assert.Equal(t, want, cfg.Orchestra.Providers["codex"])
		})
	}
}

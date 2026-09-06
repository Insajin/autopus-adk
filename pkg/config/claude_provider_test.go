package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultClaudeProviderEntry_ShipsFrontierModelAtMaxEffort pins the native
// orchestra default: the review, plan, and secure surface runs the top rung of
// the balanced role matrix at full reasoning depth.
func TestDefaultClaudeProviderEntry_ShipsFrontierModelAtMaxEffort(t *testing.T) {
	t.Parallel()

	entry := DefaultClaudeProviderEntry()
	want := []string{"--print", "--model", ClaudeFableModel, "--effort", "max"}

	assert.Equal(t, "claude", entry.Binary)
	assert.Equal(t, want, entry.Args)
	assert.Equal(t, want, entry.PaneArgs, "pane argv keeps --print: claude streams one response on both surfaces")
	assert.Equal(t, ClaudeOrchestraTimeoutSeconds, entry.Subprocess.Timeout)
	assert.Empty(t, entry.Backend)
	assert.Empty(t, entry.ModelPolicy)
	assert.False(t, entry.PromptViaArgs)
}

// TestDefaultClaudeProviderEntry_ArgvSurfacesAreIndependent guards the argv
// against slice aliasing. Callers upsert an explicit --effort into one surface
// at a time, and defaultProviderEntries holds one long-lived copy, so a shared
// backing array would let a single runtime override rewrite every consumer.
func TestDefaultClaudeProviderEntry_ArgvSurfacesAreIndependent(t *testing.T) {
	t.Parallel()

	entry := DefaultClaudeProviderEntry()
	require.Len(t, entry.Args, 5)
	entry.Args[4] = "mutated-subprocess"
	entry.PaneArgs[2] = "mutated-pane"

	assert.Equal(t, "max", entry.PaneArgs[4], "pane argv must not share the subprocess backing array")
	assert.Equal(t, ClaudeFableModel, entry.Args[2], "subprocess argv must not share the pane backing array")
	fresh := DefaultClaudeProviderEntry()
	assert.Equal(t, []string{"--print", "--model", ClaudeFableModel, "--effort", "max"}, fresh.Args)
	assert.Equal(t, []string{"--print", "--model", ClaudeFableModel, "--effort", "max"}, fresh.PaneArgs)
}

// TestUpgradeHistoricalClaudeProviderDefaults covers the migration boundary: a
// stored entry that still names a shipped historical default moves onto the
// current model policy, and everything a user chose deliberately stays put.
func TestUpgradeHistoricalClaudeProviderDefaults(t *testing.T) {
	t.Parallel()

	current := DefaultClaudeProviderEntry().Args
	shortFlagCurrent := []string{"-p", "--model", ClaudeFableModel, "--effort", "max"}

	tests := []struct {
		name         string
		entry        ProviderEntry
		wantChanged  bool
		wantArgs     []string
		wantPaneArgs []string
	}{
		{
			name: "historical high default moves to the frontier model",
			entry: ProviderEntry{
				Binary:   "claude",
				Args:     []string{"--print", "--model", "opus", "--effort", "high"},
				PaneArgs: []string{"--print", "--model", "opus", "--effort", "high"},
			},
			wantChanged:  true,
			wantArgs:     current,
			wantPaneArgs: current,
		},
		{
			name: "historical max default moves to the frontier model",
			entry: ProviderEntry{
				Binary:   "claude",
				Args:     []string{"--print", "--model", "opus", "--effort", "max"},
				PaneArgs: []string{"-p", "--model", "opus", "--effort", "max"},
			},
			wantChanged:  true,
			wantArgs:     current,
			wantPaneArgs: shortFlagCurrent,
		},
		{
			name:         "current default is left alone",
			entry:        DefaultClaudeProviderEntry(),
			wantChanged:  false,
			wantArgs:     current,
			wantPaneArgs: current,
		},
		{
			name: "extra flag marks the argv as user configuration",
			entry: ProviderEntry{
				Binary:   "claude",
				Args:     []string{"--print", "--model", "opus", "--effort", "high", "--verbose"},
				PaneArgs: []string{"--print", "--model", "opus", "--effort", "high", "--verbose"},
			},
			wantArgs:     []string{"--print", "--model", "opus", "--effort", "high", "--verbose"},
			wantPaneArgs: []string{"--print", "--model", "opus", "--effort", "high", "--verbose"},
		},
		{
			name: "full model id stays pinned",
			entry: ProviderEntry{
				Binary:   "claude",
				Args:     []string{"--print", "--model", "claude-opus-4-8", "--effort", "high"},
				PaneArgs: []string{"--print", "--model", "claude-opus-4-8", "--effort", "high"},
			},
			wantArgs:     []string{"--print", "--model", "claude-opus-4-8", "--effort", "high"},
			wantPaneArgs: []string{"--print", "--model", "claude-opus-4-8", "--effort", "high"},
		},
		{
			name:         "model-less print default pins nothing and is not upgraded",
			entry:        ProviderEntry{Binary: "claude", Args: []string{"--print"}, PaneArgs: []string{"--print"}},
			wantArgs:     []string{"--print"},
			wantPaneArgs: []string{"--print"},
		},
		{
			name: "pinned model policy is an explicit decision",
			entry: ProviderEntry{
				Binary:      "claude",
				ModelPolicy: ProviderModelPolicyPinned,
				Args:        []string{"--print", "--model", "opus", "--effort", "high"},
				PaneArgs:    []string{"--print", "--model", "opus", "--effort", "high"},
			},
			wantArgs:     []string{"--print", "--model", "opus", "--effort", "high"},
			wantPaneArgs: []string{"--print", "--model", "opus", "--effort", "high"},
		},
		{
			name: "backend routed entry carries no CLI argv contract",
			entry: ProviderEntry{
				Binary:   "claude",
				Backend:  ProviderBackendOMP,
				Args:     []string{"--print", "--model", "opus", "--effort", "high"},
				PaneArgs: []string{"--print", "--model", "opus", "--effort", "high"},
			},
			wantArgs:     []string{"--print", "--model", "opus", "--effort", "high"},
			wantPaneArgs: []string{"--print", "--model", "opus", "--effort", "high"},
		},
		{
			name: "wrapper binary owns its own flag contract",
			entry: ProviderEntry{
				Binary:   "claude-wrapper",
				Args:     []string{"--print", "--model", "opus", "--effort", "high"},
				PaneArgs: []string{"--print", "--model", "opus", "--effort", "high"},
			},
			wantArgs:     []string{"--print", "--model", "opus", "--effort", "high"},
			wantPaneArgs: []string{"--print", "--model", "opus", "--effort", "high"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			beforeArgs := append([]string(nil), tt.entry.Args...)
			beforePaneArgs := append([]string(nil), tt.entry.PaneArgs...)

			got, changed := upgradeHistoricalClaudeProviderDefaults(tt.entry)

			assert.Equal(t, tt.wantChanged, changed)
			assert.Equal(t, tt.wantArgs, got.Args)
			assert.Equal(t, tt.wantPaneArgs, got.PaneArgs)
			assert.Equal(t, beforeArgs, tt.entry.Args, "input argv must not be rewritten in place")
			assert.Equal(t, beforePaneArgs, tt.entry.PaneArgs, "input pane argv must not be rewritten in place")
		})
	}
}

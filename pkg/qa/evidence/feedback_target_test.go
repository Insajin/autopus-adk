package evidence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/antigravity"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
)

// The per-target facts in a repair prompt are only useful while they are true.
// This pins them to the platform adapters themselves, so renaming an adapter or
// its CLI binary fails here instead of shipping a confidently wrong prompt.
// InstructionDoc is not exported by the adapters; each value is the root document
// that adapter injects its managed marker section into (codex_marker.go,
// claude_markers.go, antigravity_marker.go, opencode_marker.go).
func TestSupportedFeedbackTargets_MatchPlatformAdapters(t *testing.T) {
	t.Parallel()

	for flag, want := range map[string]struct{ adapter, cli, instructions string }{
		"codex":    {codex.New().Name(), codex.New().CLIBinary(), "AGENTS.md"},
		"claude":   {claude.New().Name(), claude.New().CLIBinary(), "CLAUDE.md"},
		"gemini":   {antigravity.New().Name(), antigravity.New().CLIBinary(), "GEMINI.md"},
		"opencode": {opencode.New().Name(), opencode.New().CLIBinary(), "AGENTS.md"},
	} {
		target, ok := supportedFeedbackTargets[flag]
		require.True(t, ok, "feedback target %s must stay supported", flag)
		assert.Equal(t, flag, target.Flag)
		assert.Equal(t, want.adapter, target.Adapter)
		assert.Equal(t, want.cli, target.CLIBinary)
		assert.Equal(t, want.instructions, target.InstructionDoc)
	}
}

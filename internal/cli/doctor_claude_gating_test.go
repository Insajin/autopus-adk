package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

// claudeOnlyDoctorCheckIDs are the checks that read claude-code's own artifacts.
// .claude/settings.json is written by the claude adapter, and the parent
// .claude/rules/ scan exists because claude-code walks up for rules; its only
// remedy, isolate_rules, is a claude-only config key. A workspace that lists
// neither claude-code can satisfy neither check, so both were permanent false
// positives: the settings check warned on every run, and the parent-rules
// conflict drove the text verdict to failed with no action the user could take.
var claudeOnlyDoctorCheckIDs = []string{
	"doctor.hooks.settings",
	"doctor.hooks.configured",
	"doctor.permissions.allow",
}

const claudeRuleConflictCheckPrefix = "doctor.rule_conflict."

func TestDoctorOMPOnlyWorkspaceOmitsClaudeOnlyChecks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hermetic fake omp uses a POSIX shell script")
	}

	parent := t.TempDir()
	// A parent rule namespace that claude-code would inherit.
	require.NoError(t, os.MkdirAll(filepath.Join(parent, ".claude", "rules", "moai"), 0o755))

	root := filepath.Join(parent, "omp-project")
	require.NoError(t, os.MkdirAll(root, 0o755))
	cfg := config.DefaultFullConfig("omp-only-doctor")
	cfg.Platforms = []string{"omp"}
	require.False(t, cfg.IsolateRules,
		"the conflict has to be live: isolate_rules would downgrade it to a pass")
	require.NoError(t, config.Save(root, cfg))
	_, err := omp.NewWithRoot(root).Generate(context.Background(), cfg)
	require.NoError(t, err)

	installHermeticOMPDoctorCLI(t)
	text, envelope := runOMPDoctorEntrypoints(t, root)

	for _, absent := range []string{
		"Hooks & Permissions",
		".claude/settings.json",
		"Rule Conflicts",
		"Parent rules:",
	} {
		assert.NotContains(t, text, absent,
			"a claude-only diagnostic must not reach an omp-only workspace")
	}

	for _, check := range envelope.Checks {
		assert.NotContains(t, claudeOnlyDoctorCheckIDs, check.ID,
			"claude-only check %q must be gated out by the platform list", check.ID)
		assert.False(t, strings.HasPrefix(check.ID, claudeRuleConflictCheckPrefix),
			"parent .claude/rules conflicts are a claude-code concern, got %q", check.ID)
	}
	// The verdict is derived from the checks, so this is the FAIL clause: no
	// claude-only artifact may contribute a warning or a failure here.
	for _, check := range failingOMPDoctorChecks(envelope.Checks) {
		assert.NotContains(t, check.Detail, ".claude",
			"check %q must not fail an omp-only workspace over a claude path", check.ID)
	}

	// Same directory shape with claude-code configured: the checks are gated on
	// the platform list, not deleted.
	claudeRoot := filepath.Join(parent, "claude-project")
	require.NoError(t, os.MkdirAll(claudeRoot, 0o755))
	claudeCfg := config.DefaultFullConfig("claude-doctor")
	claudeCfg.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(claudeRoot, claudeCfg))

	claudeText, claudeEnvelope := runOMPDoctorEntrypoints(t, claudeRoot)
	assert.Contains(t, claudeText, "Hooks & Permissions")
	assert.Contains(t, claudeText, ".claude/settings.json")

	claudeIDs := make([]string, 0, len(claudeEnvelope.Checks))
	for _, check := range claudeEnvelope.Checks {
		claudeIDs = append(claudeIDs, check.ID)
	}
	assert.Contains(t, claudeIDs, "doctor.hooks.settings",
		"a claude-code workspace still gets the settings.json check")
	assert.True(t, slices.ContainsFunc(claudeIDs, func(id string) bool {
		return strings.HasPrefix(id, claudeRuleConflictCheckPrefix)
	}), "a claude-code workspace still gets the parent rule conflict check")
}

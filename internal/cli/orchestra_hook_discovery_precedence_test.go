package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	codexRelativeStopCommand = `.codex/hooks/autopus/hook-codex-stop.sh`
	claudeDynamicStopCommand = `"${CLAUDE_PROJECT_DIR:-.}"/.claude/hooks/autopus/hook-claude-stop.sh`
)

func TestHookDiscovery_RelativeManagedAssetMustBeExecutable(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		make bool
		want bool
	}{
		{"missing", 0, false, false},
		{"non executable", 0o600, true, false},
		{"executable", 0o700, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root, _ := setupHookDiscoveryProject(t)
			if tt.make {
				writeHookFixture(t, root, codexRelativeStopCommand, tt.mode)
			}
			writeHookDiscoveryFixture(t, root, ".codex/hooks.json", `{
				"hooks":{"Stop":[{"hooks":[{"command":"`+codexRelativeStopCommand+`"}]}]}
			}`)

			got := discoverHookCapabilities().capability("codex").completion
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHookDiscovery_HomeDynamicCommandUsesEffectiveProjectRoot(t *testing.T) {
	tests := []struct {
		name         string
		assetAtHome  bool
		assetProject bool
		want         bool
	}{
		{"project asset", false, true, true},
		{"home-only asset", true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, root, _ := setupHookDiscoveryProject(t)
			writeHookDiscoveryFixture(t, home, ".claude/settings.json", `{
				"hooks":{"Stop":[{"hooks":[{"command":`+strconv.Quote(claudeDynamicStopCommand)+`}]}]}
			}`)
			if tt.assetAtHome {
				writeHookFixture(t, home, ".claude/hooks/autopus/hook-claude-stop.sh", 0o700)
			}
			if tt.assetProject {
				writeHookFixture(t, root, ".claude/hooks/autopus/hook-claude-stop.sh", 0o700)
			}

			got := discoverHookCapabilities().capability("claude").completion
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHookDiscovery_NearerDisableOverridesHomeActive(t *testing.T) {
	home, root, _ := setupHookDiscoveryProject(t)
	writeClaudeStopDocument(t, home, `{
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)
	writeClaudeStopDocument(t, root, `{
		"disableAllHooks":true,
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)

	assert.False(t, discoverHookCapabilities().capability("claude").completion)
}

func TestHookDiscovery_NearerEventAbsenceKeepsHomeCapability(t *testing.T) {
	home, root, _ := setupHookDiscoveryProject(t)
	writeClaudeStopDocument(t, home, `{
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)
	writeClaudeStopDocument(t, root, `{
		"hooks":{"PreToolUse":[{"hooks":[{"command":"user hook"}]}]}
	}`)

	assert.True(t, discoverHookCapabilities().capability("claude").completion)
}

func TestHookDiscovery_ProjectHookEntryDoesNotOverrideInheritedClaudeDisable(t *testing.T) {
	home, root, _ := setupHookDiscoveryProject(t)
	writeClaudeStopDocument(t, home, `{
		"disableAllHooks":true,
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)
	writeClaudeStopDocument(t, root, `{
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)

	assert.False(t, discoverHookCapabilities().capability("claude").completion)
}

func TestHookDiscovery_NearerClaudeDisableFalseReenablesInheritedHooks(t *testing.T) {
	home, root, _ := setupHookDiscoveryProject(t)
	writeClaudeStopDocument(t, home, `{
		"disableAllHooks":true,
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)
	writeClaudeStopDocument(t, root, `{"disableAllHooks":false,"hooks":{}}`)

	assert.True(t, discoverHookCapabilities().capability("claude").completion)
}

func TestHookDiscovery_ClaudeEntryEnabledFalseIsNotADisableOracle(t *testing.T) {
	home, root, _ := setupHookDiscoveryProject(t)
	writeClaudeStopDocument(t, home, `{
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)
	writeClaudeStopDocument(t, root, `{
		"hooks":{"Stop":[{"enabled":false,"hooks":[{"command":"autopus hook stop"}]}]}
	}`)

	assert.True(t, discoverHookCapabilities().capability("claude").completion)
}

func TestHookDiscovery_ClaudeProjectLocalDisableOverridesProjectSettings(t *testing.T) {
	_, root, _ := setupHookDiscoveryProject(t)
	writeClaudeStopDocument(t, root, `{
		"hooks":{
			"Stop":[{"hooks":[{"command":"autopus hook stop"}]}],
			"SessionStart":[{"hooks":[{"command":"autopus hook session-start"}]}]
		}
	}`)
	writeClaudeLocalDocument(t, root, `{"disableAllHooks":true,"hooks":{}}`)

	assert.Equal(t, hookCapability{}, discoverHookCapabilities().capability("claude"))
}

func TestHookDiscovery_ClaudeProjectLocalFalseReenablesInheritedHooks(t *testing.T) {
	_, root, _ := setupHookDiscoveryProject(t)
	writeClaudeStopDocument(t, root, `{
		"disableAllHooks":true,
		"hooks":{
			"Stop":[{"hooks":[{"command":"autopus hook stop"}]}],
			"SessionStart":[{"hooks":[{"command":"autopus hook session-start"}]}]
		}
	}`)
	writeClaudeLocalDocument(t, root, `{"disableAllHooks":false,"hooks":{}}`)

	assert.Equal(t,
		hookCapability{completion: true, startup: true},
		discoverHookCapabilities().capability("claude"),
	)
}

func TestHookDiscovery_DuplicateProjectRootIsStable(t *testing.T) {
	home, root, _ := setupHookDiscoveryProject(t)
	writeClaudeStopDocument(t, root, `{
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)

	assert.True(t, discoverHookCapabilities().capability("claude").completion)

	t.Chdir(root)
	assert.Equal(t, []string{filepath.Clean(home), filepath.Clean(root)}, hookDiscoveryRoots())
	assert.True(t, discoverHookCapabilities().capability("claude").completion)
}

func writeClaudeStopDocument(t *testing.T, root, document string) {
	t.Helper()
	writeHookDiscoveryFixture(t, root, ".claude/settings.json", document)
}

func writeClaudeLocalDocument(t *testing.T, root, document string) {
	t.Helper()
	writeHookDiscoveryFixture(t, root, ".claude/settings.local.json", document)
}

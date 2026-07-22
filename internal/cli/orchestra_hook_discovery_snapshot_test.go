package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverHookCapabilities_UnionsGlobalCWDAndNearestProjectRoot(t *testing.T) {
	home, projectRoot, cwd := setupHookDiscoveryProject(t)
	writeHookDiscoveryFixture(t, home, ".claude/settings.json", `{
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)
	writeHookDiscoveryFixture(t, cwd, ".claude/settings.json", `{
		"hooks":{"SessionStart":[{"hooks":[{"command":"autopus hook session-start"}]}]}
	}`)
	writeHookDiscoveryFixture(t, projectRoot, ".codex/hooks.json", `{
		"hooks":{
			"Stop":[{"hooks":[{"command":".codex/hooks/autopus/hook-codex-stop.sh"}]}],
			"SessionStart":[{"hooks":[{"command":".codex/hooks/autopus/hook-codex-sessionstart.sh"}]}]
		}
	}`)
	writeHookFixture(t, projectRoot, ".codex/hooks/autopus/hook-codex-stop.sh", 0o700)
	writeHookFixture(t, projectRoot, ".codex/hooks/autopus/hook-codex-sessionstart.sh", 0o700)

	discovery := discoverHookCapabilities()

	assert.Equal(t, []string{home, projectRoot, cwd}, hookDiscoveryRoots())
	assert.Equal(t, hookActive, readHookCapabilityDecision(
		hookModeCandidates(home, projectRoot)[0],
	).completion)
	assert.Equal(t, hookActive, readHookCapabilityDecision(
		hookModeCandidates(cwd, projectRoot)[0],
	).startup)
	direct := discoverHookCapabilitiesInCandidates(
		hookModeCandidates(home, projectRoot)[0],
		hookModeCandidates(cwd, projectRoot)[0],
	)
	assert.Equal(t, hookCapability{completion: true, startup: true}, direct.capability("claude"))
	assert.Equal(t, hookCapability{completion: true, startup: true}, discovery.capability("claude"))
	assert.Equal(t, hookCapability{completion: true, startup: true}, discovery.capability("codex"))
	assert.Equal(t, hookCapability{}, discovery.capability("gemini"))
	assert.Equal(t, hookCapability{}, discovery.capability("opencode"))
	assert.True(t, discovery.anyCompletion())
}

func TestDiscoverHookCapabilities_FullCurrentClaudeAndCodexInstalls(t *testing.T) {
	_, projectRoot, _ := setupHookDiscoveryProject(t)
	writeHookDiscoveryFixture(t, projectRoot, ".claude/settings.json", `{
		"hooks":{
			"Stop":[{"hooks":[{"command":".claude/hooks/autopus/hook-claude-stop.sh"}]}],
			"SessionStart":[{"hooks":[{"command":".claude/hooks/autopus/hook-claude-sessionstart.sh"}]}]
		}
	}`)
	writeHookDiscoveryFixture(t, projectRoot, ".codex/hooks.json", `{
		"hooks":{
			"Stop":[{"hooks":[{"command":".codex/hooks/autopus/hook-codex-stop.sh"}]}],
			"SessionStart":[{"hooks":[{"command":".codex/hooks/autopus/hook-codex-sessionstart.sh"}]}]
		}
	}`)
	for _, asset := range []string{
		".claude/hooks/autopus/hook-claude-stop.sh",
		".claude/hooks/autopus/hook-claude-sessionstart.sh",
		".codex/hooks/autopus/hook-codex-stop.sh",
		".codex/hooks/autopus/hook-codex-sessionstart.sh",
	} {
		writeHookFixture(t, projectRoot, asset, 0o700)
	}

	discovery := discoverHookCapabilities()

	assert.Equal(t, hookCapability{completion: true, startup: true}, discovery.capability("claude"))
	assert.Equal(t, hookCapability{completion: true, startup: true}, discovery.capability("codex"))
}

func TestDiscoverHookCapabilities_SeparatesAntigravityAndLegacyGeminiSurfaces(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		content   string
		provider  string
		unrelated string
	}{
		{
			name: "antigravity Stop hooks",
			path: ".agents/hooks.json",
			content: `{"autopus":{"enabled":true,"Stop":[{
				"command":".gemini/hooks/autopus/hook-gemini-stop.sh"
			}]}}`,
			provider:  "antigravity",
			unrelated: "gemini",
		},
		{
			name: "legacy Gemini AfterAgent settings",
			path: ".gemini/settings.json",
			content: `{"hooks":{"AfterAgent":[{"hooks":[{
				"command":".gemini/hooks/autopus/hook-gemini-afteragent.sh"
			}]}]}}`,
			provider:  "gemini",
			unrelated: "antigravity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, projectRoot, _ := setupHookDiscoveryProject(t)
			writeHookDiscoveryFixture(t, projectRoot, tt.path, tt.content)
			asset := ".gemini/hooks/autopus/hook-gemini-afteragent.sh"
			if tt.provider == "antigravity" {
				asset = ".gemini/hooks/autopus/hook-gemini-stop.sh"
			}
			writeHookFixture(t, projectRoot, asset, 0o700)

			discovery := discoverHookCapabilities()

			assert.Equal(t, hookCapability{completion: true}, discovery.capability(tt.provider))
			assert.Equal(t, hookCapability{}, discovery.capability(tt.unrelated))
		})
	}
}

func TestHookDiscoveryCapability_ResolvesKnownRuntimeAliasesWithoutCrossFamilyLeak(t *testing.T) {
	discovery := newHookDiscovery()
	discovery["claude"] = hookCapability{completion: true, startup: true}
	discovery["antigravity"] = hookCapability{completion: true}

	for _, alias := range []string{"claude-code"} {
		assert.Equal(t, discovery.capability("claude"), discovery.capability(alias), alias)
	}
	for _, alias := range []string{"antigravity", "antigravity-cli", "agy"} {
		assert.Equal(t, discovery.capability("antigravity"), discovery.capability(alias), alias)
	}
	assert.Equal(t, hookCapability{}, discovery.capability("gemini"))
	assert.Equal(t, hookCapability{}, discovery.capability("gemini-cli"))
}

func TestDiscoverHookCapabilities_RequiresManagedCommandInActualEvent(t *testing.T) {
	_, projectRoot, _ := setupHookDiscoveryProject(t)
	writeHookDiscoveryFixture(t, projectRoot, ".claude/settings.json", `{
		"description":"autopus Stop",
		"hooks":{"Stop":[{"hooks":[{"command":"user-stop-hook.sh"}]}]}
	}`)

	discovery := discoverHookCapabilities()

	assert.Equal(t, hookCapability{}, discovery.capability("claude"))
	assert.False(t, isHookModeAvailable())
}

func TestIsHookModeAvailable_RequiresCompletionEvent(t *testing.T) {
	_, projectRoot, _ := setupHookDiscoveryProject(t)
	writeHookDiscoveryFixture(t, projectRoot, ".claude/settings.json", `{
		"hooks":{"SessionStart":[{"hooks":[{"command":"autopus hook session-start"}]}]}
	}`)

	discovery := discoverHookCapabilities()

	assert.True(t, discovery.capability("claude").startup)
	assert.False(t, discovery.capability("claude").completion)
	assert.False(t, isHookModeAvailable(), "startup-only wiring must not enable completion collection")
}

func setupHookDiscoveryProject(t *testing.T) (home, projectRoot, cwd string) {
	t.Helper()
	home = t.TempDir()
	projectRoot = t.TempDir()
	cwd = filepath.Join(projectRoot, "modules", "nested")
	require.NoError(t, os.MkdirAll(cwd, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectRoot, "autopus.yaml"),
		[]byte("project: hook-discovery-test\n"),
		0o600,
	))
	t.Setenv("HOME", home)
	t.Chdir(cwd)
	return home, projectRoot, cwd
}

func writeHookDiscoveryFixture(t *testing.T, root, relativePath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

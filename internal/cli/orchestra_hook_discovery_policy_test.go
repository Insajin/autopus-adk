package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

const managedAntigravityHookCommand = `"$(git rev-parse --show-toplevel 2>/dev/null || (cd .. && pwd))/.gemini/hooks/autopus/hook-gemini-stop.sh"`

func TestHookDiscovery_BareSemanticCommandDelegatesAssetOwnership(t *testing.T) {
	assert.Equal(t, hookActive, hookCommandDecision("autopus hook stop", "", ""))
}

func TestHookDiscovery_RuntimeFamilyUsesProviderBinaryBeforeGenericName(t *testing.T) {
	_, root, _ := setupHookDiscoveryProject(t)
	writeHookFixture(t, root, ".gemini/hooks/autopus/hook-gemini-stop.sh", 0o700)
	writeHookDiscoveryFixture(t, root, ".agents/hooks.json", `{
		"autopus":{"enabled":true,"Stop":[{"command":`+
		strconv.Quote(managedAntigravityHookCommand)+`}]}
	}`)
	providers := []orchestra.ProviderConfig{
		{Name: "gemini", Binary: "agy"},
		{Name: "gemini", Binary: "gemini"},
		{Name: "antigravity", Binary: "/opt/bin/gemini-cli"},
	}

	applyDiscoveredHookCapabilities(providers, discoverHookCapabilities())

	assertProviderHook(t, providers[0], true)
	assertProviderHook(t, providers[1], false)
	assertProviderHook(t, providers[2], false)
}

func TestHookDiscovery_AntigravityHomeConfigUsesConfigRoot(t *testing.T) {
	tests := []struct {
		name         string
		assetAtHome  bool
		assetProject bool
		want         bool
	}{
		{"home asset", true, false, true},
		{"project-only asset", false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, root, _ := setupHookDiscoveryProject(t)
			writeHookDiscoveryFixture(t, home, ".agents/hooks.json", `{
				"autopus":{"enabled":true,"Stop":[{"command":`+
				strconv.Quote(managedAntigravityHookCommand)+`}]}
			}`)
			if tt.assetAtHome {
				writeHookFixture(t, home, ".gemini/hooks/autopus/hook-gemini-stop.sh", 0o700)
			}
			if tt.assetProject {
				writeHookFixture(t, root, ".gemini/hooks/autopus/hook-gemini-stop.sh", 0o700)
			}

			got := discoverHookCapabilities().capability("antigravity").completion
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHookDiscovery_WrongRuntimeEventsAreIgnored(t *testing.T) {
	_, root, _ := setupHookDiscoveryProject(t)
	writeHookDiscoveryFixture(t, root, ".agents/hooks.json", `{
		"autopus":{"AfterAgent":[{"hooks":[{"command":"autopus hook after-agent"}]}]}
	}`)
	writeHookDiscoveryFixture(t, root, ".gemini/settings.json", `{
		"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}
	}`)

	discovery := discoverHookCapabilities()

	assert.Equal(t, hookCapability{}, discovery.capability("antigravity"))
	assert.Equal(t, hookCapability{}, discovery.capability("gemini"))
}

func TestHookDiscovery_WrongManagedAssetForRuntimeIsIgnored(t *testing.T) {
	_, root, _ := setupHookDiscoveryProject(t)
	writeHookFixture(t, root, ".gemini/hooks/autopus/hook-gemini-afteragent.sh", 0o700)
	wrongCommand := `"${GEMINI_PROJECT_DIR:-.}"/.gemini/hooks/autopus/hook-gemini-afteragent.sh`
	writeHookDiscoveryFixture(t, root, ".agents/hooks.json", `{
		"autopus":{"Stop":[{"command":`+strconv.Quote(wrongCommand)+`}]}
	}`)

	assert.Equal(t, hookCapability{}, discoverHookCapabilities().capability("antigravity"))
}

func TestHookDiscovery_DisabledScopesAreInactive(t *testing.T) {
	tests := []struct {
		name string
		path string
		doc  string
	}{
		{
			"global disable", ".claude/settings.json",
			`{"disableAllHooks":true,"hooks":{"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}}`,
		},
		{
			"document disabled", ".agents/hooks.json",
			`{"enabled":false,"autopus":{"Stop":[{"command":"autopus hook stop"}]}}`,
		},
		{
			"container disabled", ".agents/hooks.json",
			`{"autopus":{"enabled":false,"Stop":[{"hooks":[{"command":"autopus hook stop"}]}]}}`,
		},
		{
			"entry disabled", ".agents/hooks.json",
			`{"autopus":{"Stop":[{"enabled":false,"hooks":[{"command":"autopus hook stop"}]}]}}`,
		},
		{
			"handler disabled", ".agents/hooks.json",
			`{"autopus":{"Stop":[{"hooks":[{"enabled":false,"command":"autopus hook stop"}]}]}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root, _ := setupHookDiscoveryProject(t)
			writeHookDiscoveryFixture(t, root, tt.path, tt.doc)

			assert.False(t, discoverHookCapabilities().anyCompletion())
		})
	}
}

func TestHookDiscovery_ManagedAssetMustBeRegularAndExecutable(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		make bool
		want bool
	}{
		{"missing", 0, false, false},
		{"present executable", 0o700, true, true},
		{"present non-executable", 0o600, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root, _ := setupHookDiscoveryProject(t)
			if tt.make {
				writeHookFixture(t, root, ".gemini/hooks/autopus/hook-gemini-stop.sh", tt.mode)
			}
			writeHookDiscoveryFixture(t, root, ".agents/hooks.json", `{
				"autopus":{"Stop":[{"command":`+
				strconv.Quote(managedAntigravityHookCommand)+`}]}
			}`)

			got := discoverHookCapabilities().capability("antigravity").completion
			assert.Equal(t, tt.want, got)
		})
	}
}

func writeHookFixture(t *testing.T, root, relativePath string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), mode))
}

func assertProviderHook(t *testing.T, provider orchestra.ProviderConfig, want bool) {
	t.Helper()
	if assert.NotNil(t, provider.HasHook) {
		assert.Equal(t, want, *provider.HasHook)
	}
}

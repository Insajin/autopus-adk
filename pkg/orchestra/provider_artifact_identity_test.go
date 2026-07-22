package orchestra

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderArtifactIdentity_CanonicalizesKnownAliasesOnly(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"claude":            "claude",
		"claude-code":       "claude",
		"antigravity":       "gemini",
		"antigravity-cli":   "gemini",
		"gemini-cli":        "gemini",
		"agy":               "gemini",
		"CustomProvider_01": "CustomProvider_01",
	}
	for provider, want := range tests {
		provider, want := provider, want
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, providerArtifactIdentity(provider))
		})
	}
}

func TestHookSession_AliasUsesCanonicalArtifactsAndPayload(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("provider-alias-artifacts")
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)

	require.NoError(t, session.WriteInputRound("claude-code", 2, "review this"))
	data, err := session.readArtifact("claude-round2-input.json")
	require.NoError(t, err)
	var input HookInput
	require.NoError(t, json.Unmarshal(data, &input))
	assert.Equal(t, "claude", input.Provider)
	assert.Equal(t, "review this", input.Prompt)
	_, err = session.statArtifact("claude-code-round2-input.json")
	assert.Error(t, err)

	require.NoError(t, session.WriteRoundCursor("claude-code", 2))
	_, err = session.statArtifact("claude-round-cursor")
	assert.NoError(t, err)
}

func TestHookSession_AliasCapabilityOverridesCanonicalProvider(t *testing.T) {
	t.Parallel()

	on, off := true, false
	session, err := NewHookSession("provider-alias-capabilities")
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)
	session.ApplyProviderHooks([]ProviderConfig{
		{Name: "claude-code", HasHook: &off, HasStartupHook: &off},
		{Name: "agy", HasHook: &on, HasStartupHook: &on},
	})

	assert.False(t, session.HasHook("claude"))
	assert.False(t, session.HasHook("claude-code"))
	assert.True(t, session.HasHook("gemini"))
	assert.True(t, session.HasHook("antigravity-cli"))
	assert.True(t, session.HasStartupHook("gemini-cli"))
	assert.Equal(t, "done-gemini-round2", buildSignalName("agy", 2))
}

func TestHookSession_AliasReadsAndResetsCanonicalLegacyArtifacts(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("provider-alias-legacy-artifacts")
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)
	require.NoError(t, session.writeJSONArtifact("gemini-result.json", HookResult{Output: "ok"}))
	require.NoError(t, session.writeArtifact("gemini-done", nil, 0o600))

	require.NoError(t, session.WaitForDone(500*time.Millisecond, "agy"))
	result, err := session.ReadResult("gemini-cli")
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Output)
	require.NoError(t, session.ResetAttempt("antigravity", 0))
	_, err = session.statArtifact("gemini-done")
	assert.Error(t, err)
	_, err = session.statArtifact("gemini-result.json")
	assert.Error(t, err)
}

package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookSession_HasStartupHook_UsesIndependentDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("startup-capability-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()

	assert.True(t, session.HasStartupHook("claude"))
	assert.True(t, session.HasStartupHook("codex"))
	assert.False(t, session.HasStartupHook("gemini"))
	assert.False(t, session.HasStartupHook("opencode"))

	on, off := true, false
	session.ApplyProviderHooks([]ProviderConfig{
		{Name: "codex", HasStartupHook: &off},
		{Name: "opencode", HasHook: &on},
		{Name: "custom-ready", HasStartupHook: &on},
	})
	assert.False(t, session.HasStartupHook("codex"))
	assert.True(t, session.HasHook("opencode"))
	assert.False(t, session.HasStartupHook("opencode"))
	assert.True(t, session.HasStartupHook("custom-ready"))
}

func TestWaitForPaneReady_CompletionOnlyHooks_UseStableScreen(t *testing.T) {
	t.Parallel()
	on := true
	tests := []struct {
		name     string
		provider ProviderConfig
		screen   string
	}{
		{name: "gemini default", provider: ProviderConfig{Name: "gemini"}, screen: "> Type your message\n"},
		{name: "opencode completion override", provider: ProviderConfig{Name: "opencode", HasHook: &on}, screen: "Ask anything\n"},
		{name: "custom completion override", provider: ProviderConfig{Name: "custom", HasHook: &on}, screen: "Ask anything\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session, err := NewHookSession("completion-only-ready-" + NewSessionID())
			require.NoError(t, err)
			defer session.Cleanup()
			session.ApplyProviderHooks([]ProviderConfig{tt.provider})
			term := &seqScreenMock{name: "cmux", screens: []string{tt.screen, tt.screen}}

			ready := waitForPaneReady(context.Background(), term, "surface:completion-only",
				SessionReadyPatterns(), time.Second, session, tt.provider.Name, 0)

			assert.True(t, session.HasHook(tt.provider.Name))
			assert.False(t, session.HasStartupHook(tt.provider.Name))
			assert.True(t, ready)
			assert.Equal(t, 2, term.readCalls)
		})
	}
}

func TestWaitForPaneReady_DefaultStartupHooks_MissingSignalFails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		provider string
		screen   string
	}{
		{provider: "claude", screen: "❯ Try \"write a test for <filepath>\"\n"},
		{provider: "codex", screen: "› Summarize recent commits\n"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			t.Parallel()
			session, err := NewHookSession("startup-signal-required-" + NewSessionID())
			require.NoError(t, err)
			defer session.Cleanup()
			term := &seqScreenMock{name: "cmux", screens: []string{tt.screen, tt.screen}}

			ready := waitForPaneReady(context.Background(), term, "surface:startup-required",
				SessionReadyPatterns(), 450*time.Millisecond, session, tt.provider, 0)

			assert.True(t, session.HasStartupHook(tt.provider))
			assert.False(t, ready, "stable screen cannot replace a required startup artifact")
		})
	}
}

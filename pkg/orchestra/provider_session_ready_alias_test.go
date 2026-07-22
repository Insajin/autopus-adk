package orchestra

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsProviderSessionReady_KnownAliasesRemainProviderSpecific(t *testing.T) {
	tests := []struct {
		provider    string
		ownPrompt   string
		otherPrompt string
	}{
		{
			provider: "claude-code", ownPrompt: "❯ Try \"write a test for <filepath>\"\n",
			otherPrompt: codexStablePrompt,
		},
		{
			provider: "gemini-cli", ownPrompt: "> Type your message\n",
			otherPrompt: "❯\n",
		},
		{
			provider: "antigravity-cli", ownPrompt: "> Type your message\n",
			otherPrompt: codexStablePrompt,
		},
		{
			provider: "agy", ownPrompt: "> Type your message\n",
			otherPrompt: "Ask anything\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			patterns := SessionReadyPatterns()
			assert.True(t, isProviderSessionReady(tt.ownPrompt, patterns, tt.provider))
			assert.False(t, isProviderSessionReady(tt.otherPrompt, patterns, tt.provider),
				"a known alias must not use the custom-provider generic fallback")
		})
	}
}

func TestIsProviderSessionReady_TrulyCustomProviderKeepsGenericFallback(t *testing.T) {
	assert.True(t, isProviderSessionReady("Ask anything\n", SessionReadyPatterns(), "my-provider"))
}

func TestStartupTimeoutFor_KnownAliasesUseCanonicalDefaults(t *testing.T) {
	tests := []struct {
		provider string
		want     time.Duration
	}{
		{provider: "claude-code", want: 15 * time.Second},
		{provider: "gemini-cli", want: 10 * time.Second},
		{provider: "antigravity", want: 10 * time.Second},
		{provider: "antigravity-cli", want: 10 * time.Second},
		{provider: "agy", want: 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			assert.Equal(t, tt.want, startupTimeoutFor(ProviderConfig{Name: tt.provider}))
		})
	}
}

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPlatformToProvider_AllCases verifies every platform-to-provider mapping
// including the unknown-platform fallback.
func TestPlatformToProvider_AllCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		platform string
		want     string
	}{
		{"claude-code", "claude"},
		{"codex", "codex"},
		{"gemini-cli", "gemini"},
		{"opencode", ""},
		{"cursor", ""},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			t.Parallel()
			got := PlatformToProvider(tt.platform)
			assert.Equal(t, tt.want, got, "PlatformToProvider(%q) must return %q", tt.platform, tt.want)
		})
	}
}

// TestContainsString_TrueAndFalse verifies containsString returns correct results.
func TestContainsString_TrueAndFalse(t *testing.T) {
	t.Parallel()

	slice := []string{"alpha", "beta", "gamma"}

	assert.True(t, containsString(slice, "alpha"), "must find 'alpha' in slice")
	assert.True(t, containsString(slice, "gamma"), "must find last element 'gamma'")
	assert.False(t, containsString(slice, "delta"), "must not find 'delta' in slice")
	assert.False(t, containsString(nil, "alpha"), "must return false for nil slice")
	assert.False(t, containsString([]string{}, "alpha"), "must return false for empty slice")
}

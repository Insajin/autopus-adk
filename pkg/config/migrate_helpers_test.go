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
		{"antigravity-cli", "gemini"},
		{"gemini-cli", "gemini"},
		{"opencode", "codex"},
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

// TestProviderToPlatform_AllCases verifies every provider-to-platform mapping.
func TestProviderToPlatform_AllCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"claude", "claude-code"},
		{"gemini", "antigravity-cli"},
		{"gemini-cli", "antigravity-cli"},
		{"agy", "antigravity-cli"},
		{"antigravity", "antigravity-cli"},
		{"codex", ""},    // already valid as platform name
		{"opencode", ""}, // already valid as platform name
		{"cursor", ""},   // no provider→platform mapping
		{"unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := ProviderToPlatform(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestMigratePlatformNames_NormalizesGemini verifies gemini is mapped to antigravity-cli.
func TestMigratePlatformNames_NormalizesGemini(t *testing.T) {
	t.Parallel()
	cfg := &HarnessConfig{
		Platforms: []string{"claude-code", "gemini"},
	}
	changed := MigratePlatformNames(cfg)
	assert.True(t, changed)
	assert.Equal(t, []string{"claude-code", "antigravity-cli"}, cfg.Platforms)
}

func TestMigratePlatformNames_MigratesLegacyGeminiCLI(t *testing.T) {
	t.Parallel()
	cfg := &HarnessConfig{
		Platforms: []string{"claude-code", "gemini-cli"},
	}
	changed := MigratePlatformNames(cfg)
	assert.True(t, changed)
	assert.Equal(t, []string{"claude-code", "antigravity-cli"}, cfg.Platforms)
}

// TestMigratePlatformNames_NoChangeWhenValid verifies no mutation on valid platform names.
func TestMigratePlatformNames_NoChangeWhenValid(t *testing.T) {
	t.Parallel()
	cfg := &HarnessConfig{
		Platforms: []string{"claude-code", "antigravity-cli"},
	}
	changed := MigratePlatformNames(cfg)
	assert.False(t, changed)
	assert.Equal(t, []string{"claude-code", "antigravity-cli"}, cfg.Platforms)
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

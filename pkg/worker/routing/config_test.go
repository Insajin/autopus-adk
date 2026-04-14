package routing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	t.Run("disabled by default", func(t *testing.T) {
		t.Parallel()
		assert.False(t, cfg.Enabled)
	})

	t.Run("default thresholds", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 200, cfg.Thresholds.SimpleMaxChars)
		assert.Equal(t, 1000, cfg.Thresholds.ComplexMinChars)
	})

	t.Run("three providers configured", func(t *testing.T) {
		t.Parallel()
		require.Len(t, cfg.Models, 3)
		assert.Contains(t, cfg.Models, "claude")
		assert.Contains(t, cfg.Models, "codex")
		assert.Contains(t, cfg.Models, "gemini")
	})

	t.Run("claude models", func(t *testing.T) {
		t.Parallel()
		m := cfg.Models["claude"]
		assert.Equal(t, "claude-sonnet-4-6", m.Simple)
		assert.Equal(t, "claude-sonnet-4-6", m.Medium)
		assert.Equal(t, "claude-opus-4-6", m.Complex)
	})

	t.Run("codex models", func(t *testing.T) {
		t.Parallel()
		m := cfg.Models["codex"]
		assert.Equal(t, "gpt-4o-mini", m.Simple)
		assert.Equal(t, "gpt-4o", m.Medium)
		assert.Equal(t, "o3", m.Complex)
	})

	t.Run("gemini models", func(t *testing.T) {
		t.Parallel()
		m := cfg.Models["gemini"]
		assert.Equal(t, "gemini-2.0-flash", m.Simple)
		assert.Equal(t, "gemini-2.5-pro", m.Medium)
		assert.Equal(t, "gemini-2.5-pro", m.Complex)
	})
}

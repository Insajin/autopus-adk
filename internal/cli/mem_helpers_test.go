package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestJoinArgs_VariousCases verifies space-joined argument construction.
func TestJoinArgs_VariousCases(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", joinArgs(nil))
	assert.Equal(t, "", joinArgs([]string{}))
	assert.Equal(t, "foo", joinArgs([]string{"foo"}))
	assert.Equal(t, "foo bar baz", joinArgs([]string{"foo", "bar", "baz"}))
}

// TestMemDisplayPath_RelativeAndFallback verifies path display logic.
func TestMemDisplayPath_RelativeAndFallback(t *testing.T) {
	t.Parallel()

	// Path inside project dir → relative forward-slash path.
	t.Run("relative path inside project", func(t *testing.T) {
		t.Parallel()

		got := memDisplayPath("/project", "/project/memory/MEMORY.md")
		assert.Equal(t, "memory/MEMORY.md", got)
	})

	// Same dir.
	t.Run("same as project dir", func(t *testing.T) {
		t.Parallel()

		got := memDisplayPath("/project", "/project/MEMORY.md")
		assert.Equal(t, "MEMORY.md", got)
	})

	// Path outside project dir → basename fallback.
	t.Run("outside project dir fallback", func(t *testing.T) {
		t.Parallel()

		got := memDisplayPath("/project", "/other/MEMORY.md")
		assert.Equal(t, "MEMORY.md", got)
	})
}

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion_Defaults(t *testing.T) {
	t.Parallel()
	// Without ldflags injection, version falls back to "dev" or module version
	v := Version()
	assert.NotEmpty(t, v, "Version should not be empty")
	assert.NotEqual(t, "unknown", Commit())
}

func TestString(t *testing.T) {
	t.Parallel()
	s := String()
	assert.Contains(t, s, "auto")
	assert.Contains(t, s, Version())
}

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion_Defaults(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "0.6.0", Version())
	assert.Equal(t, "none", Commit())
	assert.Equal(t, "unknown", Date())
}

func TestString(t *testing.T) {
	t.Parallel()
	s := String()
	assert.Contains(t, s, "auto")
	assert.Contains(t, s, "0.6.0")
}

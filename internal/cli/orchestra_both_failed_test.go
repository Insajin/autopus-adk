package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBothBackendsUnavailableError_S9 is the S9 oracle (REQ-009): the error
// message returned by bothBackendsUnavailableError must contain all three
// recovery keywords: "auto init", "cmux", and "API".
func TestBothBackendsUnavailableError_S9(t *testing.T) {
	t.Parallel()

	err := bothBackendsUnavailableError("")
	require.Error(t, err, "S9: bothBackendsUnavailableError must return a non-nil error")

	msg := err.Error()
	assert.True(t, strings.Contains(msg, "auto init"),
		"S9: error message must contain 'auto init', got: %s", msg)
	assert.True(t, strings.Contains(msg, "cmux"),
		"S9: error message must contain 'cmux', got: %s", msg)
	assert.True(t, strings.Contains(msg, "API"),
		"S9: error message must contain 'API', got: %s", msg)
}

// TestBothBackendsUnavailableError_WithDetail verifies that a non-empty detail
// string is appended to the actionable error message without removing the three
// required recovery keywords.
func TestBothBackendsUnavailableError_WithDetail(t *testing.T) {
	t.Parallel()

	detail := "revision 0: 2 provider(s) configured, 0 responses received"
	err := bothBackendsUnavailableError(detail)
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, detail, "detail must be embedded in the error message")
	assert.Contains(t, msg, "auto init")
	assert.Contains(t, msg, "cmux")
	assert.Contains(t, msg, "API")
}

// TestBothBackendsUnavailableError_NoDetail verifies the no-detail variant
// still satisfies S9.
func TestBothBackendsUnavailableError_NoDetail(t *testing.T) {
	t.Parallel()

	err := bothBackendsUnavailableError("")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), " — ", "empty detail must not append separator")
}

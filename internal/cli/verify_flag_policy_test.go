package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunVerify_StrictWithDisabledVisualGate_ReturnsUsageError(t *testing.T) {
	// Given / When
	err := runVerifyWithOptions(nil, false, true, "desktop", verifyVisualOptions{
		Enabled: false,
		Strict:  true,
	})

	// Then
	assert.ErrorContains(t, err, "--strict-visual-gate requires --visual-gate=true")
}

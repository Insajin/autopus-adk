package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// readMachineID returns a string (may be empty on minimal CI environments).
// The test asserts only that the function does not panic and returns a string.
func TestReadMachineID_ReturnsString(t *testing.T) {
	t.Parallel()

	id := readMachineID()
	// Value may be empty (no /etc/machine-id and non-darwin), that is fine.
	assert.IsType(t, "", id)
}

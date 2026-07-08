package taskid

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"simple", "task-001", false},
		{"dot and underscore", "phase_1.worker", false},
		{"empty", "", true},
		{"leading space", " task-001", true},
		{"slash", "task/001", true},
		{"shell words", "index.lock file exists", true},
		{"too long", strings.Repeat("a", 129), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tt.id)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "task ID")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

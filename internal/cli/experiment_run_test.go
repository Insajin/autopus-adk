package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExperimentRunCommand_PrintsStopReasonAndIterations(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"experiment",
		"run",
		"--max-iterations", "2",
		"--timeout", "1s",
		"--metric", "printf '{\"metric\": 1}'",
	})

	require.NoError(t, cmd.Execute())
	output := out.String()
	assert.Contains(t, output, "stop_reason=max-iterations")
	assert.Contains(t, output, "total_iterations=2")
}

func TestExperimentRunCommand_RejectsBackgroundMetricCommand(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"experiment",
		"run",
		"--max-iterations", "1",
		"--timeout", "1s",
		"--metric", `sleep 1 & echo '{"metric": 1}'`,
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metric command validation failed")
	assert.Contains(t, err.Error(), `&`)
}

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextProductSession_SeparatesOriginalAndPhaseDeliveryCommands(t *testing.T) {
	input, _, _, _ := newWorkflowContextProductFixture(t)
	input.Command = "go"
	input.DeliveryCommand = "test"
	require.NoError(t, os.WriteFile(filepath.Join(input.ProjectDir, ".autopus", "project", "scenarios.md"),
		[]byte("# Scenarios\n\n- verify managed active delivery\n"), 0o600))

	canonical, err := canonicalWorkflowContextProductInput(input)

	require.NoError(t, err)
	assert.Equal(t, "go", canonical.Command)
	assert.Equal(t, "test", canonical.DeliveryCommand)
	assert.Equal(t, "/auto go SPEC-OMP-004 --auto", canonical.OriginalTask)
}

func TestWorkflowContextProductSession_RejectsWrongOriginalOrDeliveryCommandBeforeDriver(t *testing.T) {
	for _, mutate := range []func(*WorkflowContextProductSessionInput){
		func(input *WorkflowContextProductSessionInput) { input.Command = "test" },
		func(input *WorkflowContextProductSessionInput) { input.DeliveryCommand = "fix" },
	} {
		input, runtime, _, factory := newWorkflowContextProductFixture(t)
		mutate(&input)
		_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)
		require.Error(t, err)
		assert.Zero(t, factory.calls)
	}
}

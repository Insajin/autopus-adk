package cli

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A session that exposes any tool outside the allowlist (MCP, custom, or
// extension tools arrive through discovery, not argv) must fail closed before
// the prompt is sent.
func TestOMPReviewBackend_FailsClosedWhenSessionLeaksTools(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	projectDir, logPath := configureOMPReviewRPCFixture(t, "leaked-tool")
	response, err := executeOMPReviewFixture(t, projectDir, 5*time.Second)
	require.Error(t, err)
	require.NotNil(t, response)
	assert.Equal(t, 1, response.ExitCode)
	assert.Contains(t, response.Error, "outside the allowlist: mcp__filesystem_delete")
	_, _, commands := splitOMPReviewRPCRecords(t, readOMPReviewRPCRecords(t, logPath))
	assert.NotContains(t, ompReviewCommandTypes(commands), "prompt")
	assertOMPReviewFixtureRuntimeRemoved(t, logPath)
}

// A turn the provider ended with an error is a provider failure with its
// status and message, never "empty output".
func TestOMPReviewBackend_ReportsProviderTurnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	projectDir, logPath := configureOMPReviewRPCFixture(t, "provider-error")
	response, err := executeOMPReviewFixture(t, projectDir, 5*time.Second)
	require.Error(t, err)
	require.NotNil(t, response)
	assert.False(t, response.EmptyOutput)
	assert.Equal(t, 1, response.ExitCode)
	assert.Contains(t, response.Error, "provider error status 404: 404 model: gone")
	assert.Equal(t, "provider_error", structuredFailureClass(err))
	starts, _, _ := splitOMPReviewRPCRecordsAll(t, readOMPReviewRPCRecords(t, logPath))
	assert.Len(t, starts, 1, "a permanent provider error is not retried")
	assertOMPReviewFixtureRuntimeRemoved(t, logPath)
}

// A transient provider error gets a fresh session on the same pinned model;
// the second attempt's reply is the response.
func TestOMPReviewBackend_RetriesTransientProviderErrorWithFreshSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	original := ompReviewBackoffUnit
	ompReviewBackoffUnit = 10 * time.Millisecond
	t.Cleanup(func() { ompReviewBackoffUnit = original })
	projectDir, logPath := configureOMPReviewRPCFixture(t, "provider-error-once")
	response, err := executeOMPReviewFixture(t, projectDir, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "review output", response.Output)
	assert.Equal(t, "omp", response.ExecutedBackend)
	starts, overlays, commands := splitOMPReviewRPCRecordsAll(t, readOMPReviewRPCRecords(t, logPath))
	require.Len(t, starts, 2, "one fresh process per attempt")
	require.Len(t, overlays, 2)
	for _, overlay := range overlays {
		assert.NoFileExists(t, overlay.Path)
		assert.NoDirExists(t, filepath.Dir(overlay.Path))
	}
	assert.NotEqual(t, starts[0].PID, starts[1].PID)
	assert.NotEqual(t, flagValue(t, starts[0].Args, "--session-dir"), flagValue(t, starts[1].Args, "--session-dir"))
	assert.Equal(t, 2, len(ompReviewCommandsOfType(commands, "prompt")))
	for _, command := range ompReviewCommandsOfType(commands, "set_model") {
		assert.Equal(t, "openai", command.Provider)
		assert.Equal(t, "model", command.ModelID)
	}
}

// The session must answer with the pinned model; a drifted identity fails closed.
func TestOMPReviewBackend_FailsClosedOnExecutedModelDrift(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	projectDir, logPath := configureOMPReviewRPCFixture(t, "model-drift")
	response, err := executeOMPReviewFixture(t, projectDir, 5*time.Second)
	require.Error(t, err)
	require.NotNil(t, response)
	assert.Equal(t, 1, response.ExitCode)
	assert.Contains(t, response.Error, "executed model mismatch: want openai/model got openai/claude-sonnet-5")
	assert.Equal(t, "provider_model_error", structuredFailureClass(err))
	assertOMPReviewFixtureRuntimeRemoved(t, logPath)
}

func splitOMPReviewRPCRecordsAll(
	t *testing.T,
	records []ompReviewRPCRecord,
) (starts, overlays, commands []ompReviewRPCRecord) {
	t.Helper()
	for _, record := range records {
		switch record.Kind {
		case "start":
			starts = append(starts, record)
		case "overlay":
			overlays = append(overlays, record)
		case "command":
			commands = append(commands, record)
		}
	}
	return starts, overlays, commands
}

func ompReviewCommandsOfType(commands []ompReviewRPCRecord, commandType string) []ompReviewRPCRecord {
	var matched []ompReviewRPCRecord
	for _, command := range commands {
		if command.Type == commandType {
			matched = append(matched, command)
		}
	}
	return matched
}

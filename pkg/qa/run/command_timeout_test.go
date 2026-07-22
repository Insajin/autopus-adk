package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func TestRunCommandReportsTimeoutDuration(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "slow")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsleep 1\n"), 0o755))

	result := runCommand(dir, journey.Pack{
		ID:      "slow",
		Adapter: journey.AdapterRef{ID: "custom-command"},
		Command: journey.Command{Argv: []string{script}, CWD: ".", Timeout: "10ms"},
	}, filepath.Join(dir, "artifacts"))

	assert.Equal(t, "blocked", result.Status)
	assert.Equal(t, -1, result.ExitCode)
	assert.Contains(t, result.FailureSummary, "timeout after 10ms")
}

func TestCommandTimeoutCannotOutliveStaleCacheLease(t *testing.T) {
	assert.Equal(t, journey.MaxCommandTimeout, commandTimeout("48h"))
	assert.Greater(t, staleCommandGoCacheAge, journey.MaxCommandTimeout)
}

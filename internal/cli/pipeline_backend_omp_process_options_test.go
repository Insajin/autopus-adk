package cli

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartPipelineOMPProcess_DefaultOptionsPreserveLegacyArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	config, logPath := pipelineOMPBackendTestConfig(t)
	normalized, err := normalizePipelineOMPBackendConfig(config)
	require.NoError(t, err)

	process, err := startPipelineOMPProcess(context.Background(), normalized)
	require.NoError(t, err)
	require.NoError(t, process.Close())

	starts, _ := pipelineOMPRPCRecordsByKind(readPipelineOMPRPCRecords(t, logPath))
	require.Len(t, starts, 1)
	args := ompReviewProcessArgs(t, starts[0].Args)
	sessionDir := flagValue(t, args, "--session-dir")
	assert.Equal(t, []string{
		"--mode", "rpc", "--no-session", "--no-extensions", "--session-dir", sessionDir,
	}, args)
	assertPipelineOMPRuntimeEmpty(t, normalized.RuntimeBase)
}

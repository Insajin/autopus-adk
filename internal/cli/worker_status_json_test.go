package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workerSetup "github.com/insajin/autopus-adk/pkg/worker/setup"
)

func TestBuildWorkerStatusWarnings_AllWarnings(t *testing.T) {
	t.Parallel()

	warnings := buildWorkerStatusWarnings(workerSetup.WorkerStatus{})

	require.Len(t, warnings, 3)
	assert.Equal(t, "worker_not_configured", warnings[0].Code)
	assert.Equal(t, "worker_auth_invalid", warnings[1].Code)
	assert.Equal(t, "worker_daemon_stopped", warnings[2].Code)
}

func TestBuildWorkerStatusWarnings_HealthyWorker(t *testing.T) {
	t.Parallel()

	warnings := buildWorkerStatusWarnings(workerSetup.WorkerStatus{
		Configured:    true,
		AuthValid:     true,
		DaemonRunning: true,
	})

	assert.Empty(t, warnings)
}

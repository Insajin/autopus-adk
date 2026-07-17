package worker

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/controlplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerLoopStart_RefusesUnsignedControlPlaneByDefault is part of the S4
// oracle for REQ-004/REQ-007: WorkerLoop.Start must return the signing-secret
// enforcement error, naming the env var without leaking any secret value,
// before acquiring the PID lock or connecting to the a2a broker, when
// neither the signing secret nor the unsigned opt-out are set.
func TestWorkerLoopStart_RefusesUnsignedControlPlaneByDefault(t *testing.T) {
	t.Setenv(controlplane.PolicySigningSecretEnv, "")
	t.Setenv(controlplane.AllowUnsignedControlPlaneEnv, "")

	wl := NewWorkerLoop(LoopConfig{})

	err := wl.Start(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), controlplane.PolicySigningSecretEnv)
	assert.Nil(t, wl.pidLock, "Start must return before acquiring the PID lock or connecting to the broker")
}

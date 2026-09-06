//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package omp

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOMPReadinessRPCDrainsFastExitOutput(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	// A single scheduler thread exposes exit racing with the pipe readers.
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	want := providerFreeRPCFixture(t)
	for attempt := 0; attempt < 32; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c",
			`cat >/dev/null; printf '%s' "$1"`, "readiness-fixture", string(want))
		got, err := runOMPReadinessRPCCommand(ctx, cmd, ompProviderFreeRPCInput(), len(want)+1024)
		cancel()
		require.NoError(t, err, "attempt %d", attempt)
		require.Equal(t, string(want), string(got), "fast exit must not discard RPC frames (attempt %d)", attempt)
	}
}

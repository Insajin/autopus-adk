// Package reaper_test tests zombie process detection and reaping.
package reaper_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReaper_Start_PeriodicCheck verifies that the reaper runs periodic checks.
func TestReaper_Start_PeriodicCheck(t *testing.T) {
	t.Parallel()

	// Given: a reaper configured with a short interval and an atomic counter
	var callCount atomic.Int32
	r := reaper.New(reaper.Config{
		Interval: 50 * time.Millisecond,
		OnReap:   func() { callCount.Add(1) },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// When: starting the reaper and waiting for multiple intervals
	err := r.Start(ctx)
	require.NoError(t, err)

	<-ctx.Done()
	// Wait for the reaper goroutine to fully exit before reading the counter.
	_ = r.Wait()

	// Then: the checker was called at least twice (periodic execution confirmed)
	assert.GreaterOrEqual(t, int(callCount.Load()), 2,
		"reaper must call checker at least twice within 200ms at 50ms interval")
}

// TestReaper_Stop_Graceful verifies that the reaper stops cleanly when context is cancelled.
func TestReaper_Stop_Graceful(t *testing.T) {
	t.Parallel()

	// Given: a running reaper
	r := reaper.New(reaper.Config{
		Interval: 100 * time.Millisecond,
		OnReap:   func() {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	err := r.Start(ctx)
	require.NoError(t, err)

	// When: cancelling the context
	cancel()
	stopErr := r.Wait()

	// Then: Wait returns without error (graceful stop)
	assert.NoError(t, stopErr, "reaper must stop gracefully on context cancellation")
}

// TestReaper_ReapZombie verifies that the reaper detects and reaps a zombie process.
func TestReaper_ReapZombie(t *testing.T) {
	t.Parallel()

	// Given: a reaper with a mock zombie detector that reports one zombie
	// Use atomic flag to avoid data race on the reaped PID value.
	var reapedPID atomic.Int32
	detector := &mockZombieDetector{zombiePIDs: []int{12345}}
	r := reaper.New(reaper.Config{
		Interval: 10 * time.Millisecond,
		Detector: detector,
		OnReapPID: func(pid int) {
			reapedPID.Store(int32(pid))
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// When: starting the reaper
	err := r.Start(ctx)
	require.NoError(t, err)

	<-ctx.Done()
	// Wait for the reaper goroutine to fully exit before reading reapedPID.
	_ = r.Wait()

	// Then: the zombie PID was reaped
	assert.Equal(t, int32(12345), reapedPID.Load(),
		"zombie PID 12345 must be reaped")
}

// mockZombieDetector is a test double for zombie process detection.
type mockZombieDetector struct {
	zombiePIDs []int
}

func (m *mockZombieDetector) DetectZombies() []int {
	return m.zombiePIDs
}

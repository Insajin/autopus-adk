// Package reaper provides zombie process detection and reaping for worker subprocesses.
package reaper

import (
	"context"
	"sync"
	"time"
)

// ZombieDetector detects zombie child processes.
type ZombieDetector interface {
	// DetectZombies returns a list of zombie PIDs.
	DetectZombies() []int
}

// Config holds configuration for the Reaper.
type Config struct {
	// Interval is how often the reaper checks for zombies. Defaults to 30s if zero.
	Interval time.Duration

	// OnReap is called on each reap cycle (before per-PID callbacks).
	// Used for periodic-check testing without a detector.
	OnReap func()

	// Detector provides zombie PID discovery. If nil, the default syscall-based
	// detector is used on Unix, and a no-op detector is used elsewhere.
	Detector ZombieDetector

	// OnReapPID is called when a zombie PID is reaped. Optional.
	OnReapPID func(pid int)
}

// Reaper periodically detects and reaps zombie child processes.
type Reaper struct {
	cfg    Config
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Reaper with the given configuration.
func New(cfg Config) *Reaper {
	if cfg.Interval <= 0 {
		// @AX:NOTE[AUTO]: magic constant — 30s default zombie scan interval; tune for high-throughput subprocess workloads
		cfg.Interval = 30 * time.Second
	}
	if cfg.Detector == nil {
		cfg.Detector = newDefaultDetector()
	}
	return &Reaper{cfg: cfg}
}

// Start begins the reaper goroutine, which runs until ctx is cancelled.
// Returns an error if the reaper is already running.
// @AX:ANCHOR[AUTO]: public lifecycle API — Start/Wait form the goroutine lifecycle contract; callers (loop_lifecycle.go) depend on this pair
func (r *Reaper) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	r.wg.Add(1)
	go r.run(ctx)
	return nil
}

// Wait blocks until the reaper goroutine exits and returns any error.
func (r *Reaper) Wait() error {
	r.wg.Wait()
	return nil
}

// run is the main reaper loop.
func (r *Reaper) run(ctx context.Context) {
	defer r.wg.Done()
	defer r.cancel()

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check ctx again before invoking callbacks so that a tick
			// arriving simultaneously with cancellation does not race with
			// the caller reading shared variables after <-ctx.Done().
			select {
			case <-ctx.Done():
				return
			default:
			}
			r.reapZombies()
		}
	}
}

// reapZombies calls the OnReap hook and reaps any zombie PIDs found by the detector.
func (r *Reaper) reapZombies() {
	if r.cfg.OnReap != nil {
		r.cfg.OnReap()
	}

	if r.cfg.Detector == nil {
		return
	}

	pids := r.cfg.Detector.DetectZombies()
	for _, pid := range pids {
		reapPID(pid)
		if r.cfg.OnReapPID != nil {
			r.cfg.OnReapPID(pid)
		}
	}
}

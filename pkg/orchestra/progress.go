package orchestra

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var progressHeartbeatInterval = 30 * time.Second

// ProviderStatus represents a provider's execution state.
type ProviderStatus int

const (
	StatusPending ProviderStatus = iota
	StatusRunning
	StatusDone
	StatusFailed
)

// String returns a display label for the provider status.
func (s ProviderStatus) String() string {
	switch s {
	case StatusPending:
		return "⏳"
	case StatusRunning:
		return "⏳"
	case StatusDone:
		return "✓"
	case StatusFailed:
		return "✗"
	default:
		return "?"
	}
}

// ProgressTracker displays real-time provider execution status.
type ProgressTracker struct {
	mu        sync.Mutex
	providers map[string]*providerState
	order     []string
	writer    io.Writer
	isTTY     bool
	startTime time.Time
	rendered  bool
	logged    map[string]ProviderStatus
}

type providerState struct {
	status  ProviderStatus
	started time.Time
	elapsed time.Duration
}

// NewProgressTracker creates a tracker for the given provider names.
func NewProgressTracker(providerNames []string) *ProgressTracker {
	providers := make(map[string]*providerState, len(providerNames))
	for _, name := range providerNames {
		providers[name] = &providerState{status: StatusPending}
	}
	return &ProgressTracker{
		providers: providers,
		order:     providerNames,
		writer:    os.Stderr,
		isTTY:     isTerminal(),
		startTime: time.Now(),
		logged:    make(map[string]ProviderStatus, len(providerNames)),
	}
}

// StartHeartbeat renders periodic progress while providers are still running.
func (pt *ProgressTracker) StartHeartbeat(ctx context.Context, interval time.Duration) func() {
	if interval <= 0 {
		return func() {}
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				pt.RenderHeartbeat()
			}
		}
	}()
	return cancel
}

// MarkRunning updates a provider to running state.
func (pt *ProgressTracker) MarkRunning(name string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if s, ok := pt.providers[name]; ok {
		s.status = StatusRunning
		s.started = time.Now()
	}
	pt.render()
}

// MarkDone updates a provider to done state.
func (pt *ProgressTracker) MarkDone(name string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if s, ok := pt.providers[name]; ok {
		s.status = StatusDone
		s.elapsed = time.Since(s.started)
	}
	pt.render()
}

// MarkFailed updates a provider to failed state.
func (pt *ProgressTracker) MarkFailed(name string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if s, ok := pt.providers[name]; ok {
		s.status = StatusFailed
		s.elapsed = time.Since(s.started)
	}
	pt.render()
}

// render writes the current status to the writer.
func (pt *ProgressTracker) render() {
	if pt.isTTY {
		pt.renderTTY()
	} else {
		pt.renderLog()
	}
}

// renderTTY renders with ANSI cursor control for in-place updates.
func (pt *ProgressTracker) renderTTY() {
	// Move cursor up by number of providers, then overwrite.
	lines := len(pt.order)
	if pt.rendered && lines > 0 {
		fmt.Fprintf(pt.writer, "\033[%dA", lines)
	}
	for _, name := range pt.order {
		s := pt.providers[name]
		elapsed := s.elapsed
		if s.status == StatusRunning {
			elapsed = time.Since(s.started)
		}
		fmt.Fprintf(pt.writer, "\033[2K  %s %-12s %6.1fs\n",
			s.status.String(), name, elapsed.Seconds())
	}
	pt.rendered = true
}

// renderLog renders as structured log lines for non-TTY environments.
func (pt *ProgressTracker) renderLog() {
	for _, name := range pt.order {
		s := pt.providers[name]
		if s.status == StatusPending {
			continue
		}
		if logged, ok := pt.logged[name]; ok && logged == s.status {
			continue
		}
		elapsed := s.elapsed
		if s.status == StatusRunning {
			elapsed = time.Since(s.started)
		}
		fmt.Fprintf(pt.writer, "[%s] %s: %s %.1fs\n",
			s.status.String(), name, progressStatusLabel(s.status), elapsed.Seconds())
		pt.logged[name] = s.status
	}
}

// RenderHeartbeat writes elapsed time for currently running providers.
func (pt *ProgressTracker) RenderHeartbeat() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.isTTY {
		pt.renderTTY()
		return
	}
	for _, name := range pt.order {
		s := pt.providers[name]
		if s.status != StatusRunning {
			continue
		}
		fmt.Fprintf(pt.writer, "[%s] %s: running %.1fs (still waiting)\n",
			s.status.String(), name, time.Since(s.started).Seconds())
	}
}

func progressStatusLabel(status ProviderStatus) string {
	switch status {
	case StatusRunning:
		return "running"
	case StatusDone:
		return "done"
	case StatusFailed:
		return "failed"
	default:
		return "pending"
	}
}

// isTerminal checks if stderr is a terminal.
func isTerminal() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

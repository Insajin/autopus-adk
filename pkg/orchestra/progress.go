package orchestra

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

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
	}
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
	if lines > 0 {
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
}

// renderLog renders as structured log lines for non-TTY environments.
func (pt *ProgressTracker) renderLog() {
	for _, name := range pt.order {
		s := pt.providers[name]
		if s.status == StatusDone || s.status == StatusFailed {
			fmt.Fprintf(pt.writer, "[%s] %s: %.1fs\n",
				s.status.String(), name, s.elapsed.Seconds())
		}
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

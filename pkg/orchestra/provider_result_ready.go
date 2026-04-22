package orchestra

import (
	"errors"
	"strings"
	"time"
)

var errResultReady = errors.New("provider result ready")

const defaultResultReadyGrace = 5 * time.Second

type resultReadyMonitor struct {
	patterns []string
	grace    time.Duration
	buffers  []*fastFailBuffer
}

func newResultReadyMonitor(provider ProviderConfig, buffers ...*fastFailBuffer) *resultReadyMonitor {
	patterns := normalizeResultReadyPatterns(provider.ResultReadyPatterns)
	if len(patterns) == 0 {
		return nil
	}

	grace := provider.ResultReadyGrace
	if grace <= 0 {
		grace = defaultResultReadyGrace
	}

	return &resultReadyMonitor{
		patterns: patterns,
		grace:    grace,
		buffers:  buffers,
	}
}

func normalizeResultReadyPatterns(patterns []string) []string {
	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, strings.ToLower(trimmed))
	}
	return normalized
}

func (b *fastFailBuffer) Snapshot() (string, time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String(), b.lastWrite
}

func (m *resultReadyMonitor) ShouldTerminate(now time.Time) bool {
	if m == nil {
		return false
	}

	var latestWrite time.Time
	var combined strings.Builder
	for _, buf := range m.buffers {
		snapshot, lastWrite := buf.Snapshot()
		if snapshot != "" {
			combined.WriteString(snapshot)
			combined.WriteByte('\n')
		}
		if lastWrite.After(latestWrite) {
			latestWrite = lastWrite
		}
	}
	if latestWrite.IsZero() || now.Sub(latestWrite) < m.grace {
		return false
	}

	lower := strings.ToLower(combined.String())
	for _, pattern := range m.patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

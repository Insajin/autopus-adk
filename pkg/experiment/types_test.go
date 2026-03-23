package experiment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	assert.Equal(t, 50, cfg.MaxIterations, "default MaxIterations should be 50")
	assert.Equal(t, 10, cfg.CircuitBreakerN, "default CircuitBreakerN should be 10")
	assert.Equal(t, 5*time.Minute, cfg.ExperimentTimeout, "default ExperimentTimeout should be 5m")
	assert.Equal(t, 1, cfg.MetricRuns, "default MetricRuns should be 1")
	assert.InDelta(t, 0.001, cfg.SimplicityThreshold, 1e-9, "default SimplicityThreshold should be 0.001")
	assert.Equal(t, Minimize, cfg.Direction, "default Direction should be Minimize")
}

func TestDirectionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dir      Direction
		expected string
	}{
		{Minimize, "minimize"},
		{Maximize, "maximize"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.dir.String())
		})
	}
}

func TestDirectionIsBetter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dir      Direction
		current  float64
		best     float64
		expected bool
	}{
		{"minimize: current < best", Minimize, 0.5, 1.0, true},
		{"minimize: current > best", Minimize, 1.5, 1.0, false},
		{"minimize: current == best", Minimize, 1.0, 1.0, false},
		{"maximize: current > best", Maximize, 1.5, 1.0, true},
		{"maximize: current < best", Maximize, 0.5, 1.0, false},
		{"maximize: current == best", Maximize, 1.0, 1.0, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.dir.IsBetter(tc.current, tc.best))
		})
	}
}

func TestResultFields(t *testing.T) {
	t.Parallel()

	r := Result{
		Iteration:    1,
		CommitHash:   "abc123",
		MetricValue:  42.0,
		MetricKey:    "latency",
		Unit:         "ms",
		Status:       "keep",
		Description:  "test result",
		LinesChanged: 5,
		Timestamp:    time.Now(),
	}

	assert.Equal(t, 1, r.Iteration)
	assert.Equal(t, "abc123", r.CommitHash)
	assert.Equal(t, 42.0, r.MetricValue)
	assert.Equal(t, "keep", r.Status)
}

func TestExperimentSummaryFields(t *testing.T) {
	t.Parallel()

	s := ExperimentSummary{
		TotalIterations: 10,
		KeepCount:       6,
		DiscardCount:    4,
		BaselineMetric:  100.0,
		BestMetric:      85.0,
		BestIteration:   7,
		Top5:            []Result{},
	}

	assert.Equal(t, 10, s.TotalIterations)
	assert.Equal(t, 6, s.KeepCount)
	assert.Equal(t, 85.0, s.BestMetric)
}

func TestMetricOutputFields(t *testing.T) {
	t.Parallel()

	m := MetricOutput{
		Metric: 1.23,
		Unit:   "ms",
		Metadata: map[string]any{
			"p50": 1.0,
			"p99": 2.5,
		},
	}

	assert.Equal(t, 1.23, m.Metric)
	assert.Equal(t, "ms", m.Unit)
	assert.Equal(t, 1.0, m.Metadata["p50"])
}

package experiment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeResult(iter int, status string, metric float64) Result {
	return Result{
		Iteration:   iter,
		CommitHash:  "abc" + string(rune('0'+iter)),
		MetricValue: metric,
		MetricKey:   "latency",
		Unit:        "ms",
		Status:      status,
		Description: "test result",
		Timestamp:   time.Now(),
	}
}

func TestNewRecorder(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()
	require.NotNil(t, rec)
}

func TestRecorder_RecordAndSummary(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()

	rec.Record(makeResult(1, "keep", 80.0))
	rec.Record(makeResult(2, "discard", 95.0))
	rec.Record(makeResult(3, "keep", 75.0))
	rec.Record(makeResult(4, "crash", 0.0))
	rec.Record(makeResult(5, "discard", 90.0))

	summary := rec.Summary()

	assert.Equal(t, 5, summary.TotalIterations)
	assert.Equal(t, 2, summary.KeepCount)
	assert.Equal(t, 2, summary.DiscardCount)
	assert.Equal(t, 75.0, summary.BestMetric)
	assert.Equal(t, 3, summary.BestIteration)
}

func TestRecorder_SummaryEmpty(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()
	summary := rec.Summary()

	assert.Equal(t, 0, summary.TotalIterations)
	assert.Equal(t, 0, summary.KeepCount)
	assert.Equal(t, 0, summary.DiscardCount)
}

func TestRecorder_Top5(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()

	// Add 8 keep results with varying metrics
	for i := 1; i <= 8; i++ {
		rec.Record(makeResult(i, "keep", float64(100-i*5)))
	}

	summary := rec.Summary()

	// Top5 should contain at most 5 results
	assert.LessOrEqual(t, len(summary.Top5), 5)
	assert.Equal(t, 5, len(summary.Top5), "should return exactly 5 top results when more than 5 keep results exist")
}

func TestRecorder_RecentHistory(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()

	for i := 1; i <= 10; i++ {
		rec.Record(makeResult(i, "keep", float64(i*10)))
	}

	history := rec.RecentHistory(5)
	assert.Len(t, history, 5, "RecentHistory(5) should return 5 results")

	// The most recent should be iteration 10
	assert.Equal(t, 10, history[len(history)-1].Iteration)
}

func TestRecorder_RecentHistoryFewerThanN(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()
	rec.Record(makeResult(1, "keep", 10.0))
	rec.Record(makeResult(2, "discard", 20.0))

	history := rec.RecentHistory(10)
	assert.Len(t, history, 2, "should return all results when fewer than N exist")
}

func TestRecorder_RecentHistoryZero(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()
	history := rec.RecentHistory(5)
	assert.Empty(t, history, "empty recorder should return empty history")
}

func TestRecorder_BaselineMetric(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()
	rec.SetBaseline(100.0)

	rec.Record(makeResult(1, "keep", 80.0))

	summary := rec.Summary()
	assert.Equal(t, 100.0, summary.BaselineMetric)
}

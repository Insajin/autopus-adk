package experiment

import (
	"math"
	"sort"
)

// Recorder accumulates experiment results in memory and computes summaries.
type Recorder struct {
	results  []Result
	baseline float64
}

// NewRecorder creates an empty in-memory Recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// SetBaseline sets the baseline metric value used in Summary.
func (r *Recorder) SetBaseline(baseline float64) {
	r.baseline = baseline
}

// Record appends a result to the in-memory store.
func (r *Recorder) Record(result Result) {
	r.results = append(r.results, result)
}

// Summary computes an ExperimentSummary from all recorded results.
// Best metric is determined by minimum value among "keep" results.
func (r *Recorder) Summary() ExperimentSummary {
	s := ExperimentSummary{
		TotalIterations: len(r.results),
		BaselineMetric:  r.baseline,
		BestMetric:      math.Inf(1),
	}

	var keepResults []Result

	for _, res := range r.results {
		switch res.Status {
		case "keep":
			s.KeepCount++
			keepResults = append(keepResults, res)
			if res.MetricValue < s.BestMetric {
				s.BestMetric = res.MetricValue
				s.BestIteration = res.Iteration
			}
		case "discard":
			s.DiscardCount++
		}
	}

	// If no keep results, reset BestMetric to zero.
	if s.BestMetric == math.Inf(1) {
		s.BestMetric = 0
	}

	// Build Top5: best (lowest) 5 keep results by MetricValue.
	sorted := make([]Result, len(keepResults))
	copy(sorted, keepResults)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].MetricValue < sorted[j].MetricValue
	})
	if len(sorted) > 5 {
		sorted = sorted[:5]
	}
	s.Top5 = sorted

	return s
}

// RecentHistory returns the last n results (all statuses).
// If n > len(results), all results are returned.
func (r *Recorder) RecentHistory(n int) []Result {
	if len(r.results) == 0 {
		return nil
	}
	if n >= len(r.results) {
		out := make([]Result, len(r.results))
		copy(out, r.results)
		return out
	}
	out := make([]Result, n)
	copy(out, r.results[len(r.results)-n:])
	return out
}

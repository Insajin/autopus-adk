package report

import "time"

// minBarPercent keeps a very short journey visible on the timeline instead of
// collapsing it to an invisible sliver.
const minBarPercent = 0.75

// applyTimeline positions every journey on a shared axis as CSS percentages.
// Journeys without parseable timestamps get a zero-width bar and the template
// falls back to the duration column.
func applyTimeline(views []JourneyView) ([]JourneyView, Timeline) {
	var start, end time.Time
	for _, view := range views {
		s, sok := parseTime(view.StartedAt)
		e, eok := parseTime(view.EndedAt)
		if sok && (start.IsZero() || s.Before(start)) {
			start = s
		}
		if eok && (end.IsZero() || e.After(end)) {
			end = e
		}
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return views, Timeline{}
	}
	span := end.Sub(start)
	for i := range views {
		s, sok := parseTime(views[i].StartedAt)
		e, eok := parseTime(views[i].EndedAt)
		if !sok || !eok {
			continue
		}
		views[i].Bar = TimelineBar{
			OffsetPercent: percent(s.Sub(start), span),
			WidthPercent:  maxFloat(percent(e.Sub(s), span), minBarPercent),
		}
	}
	return views, Timeline{
		StartedAt: start.UTC().Format(time.RFC3339),
		EndedAt:   end.UTC().Format(time.RFC3339),
		SpanMS:    span.Milliseconds(),
	}
}

func percent(part, whole time.Duration) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func spanMS(startedAt, endedAt string) int64 {
	start, sok := parseTime(startedAt)
	end, eok := parseTime(endedAt)
	if !sok || !eok || end.Before(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}

func parseTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

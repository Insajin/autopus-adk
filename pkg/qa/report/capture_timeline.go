package report

import (
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
)

// applyCaptureTimeline lays the filmstrip out against the journey's own span so
// a step's width is readable even when the journey itself is a sliver of the
// report-wide axis. Timestamps win; a producer that only reported durations
// still gets a back-to-back sequence.
func applyCaptureTimeline(views []CaptureStepView, index capture.Index) {
	if positionByTimestamp(views, index) {
		return
	}
	positionByDuration(views)
}

func positionByTimestamp(views []CaptureStepView, index capture.Index) bool {
	start, end, ok := captureSpan(index)
	if !ok {
		return false
	}
	span := end.Sub(start)
	positioned := false
	for i := range views {
		from, fromOK := parseTime(index.Steps[i].StartedAt)
		to, toOK := parseTime(index.Steps[i].EndedAt)
		if !fromOK || !toOK {
			continue
		}
		views[i].Bar = TimelineBar{
			OffsetPercent: percent(from.Sub(start), span),
			WidthPercent:  maxFloat(percent(to.Sub(from), span), minBarPercent),
		}
		positioned = true
	}
	return positioned
}

// positionByDuration stacks steps end to end in declared order, which the dense
// 1..n order guarantee makes a faithful sequence.
func positionByDuration(views []CaptureStepView) {
	var total int64
	for _, view := range views {
		total += view.DurationMS
	}
	if total <= 0 {
		return
	}
	var cursor int64
	for i := range views {
		views[i].Bar = TimelineBar{
			OffsetPercent: float64(cursor) / float64(total) * 100,
			WidthPercent:  maxFloat(float64(views[i].DurationMS)/float64(total)*100, minBarPercent),
		}
		cursor += views[i].DurationMS
	}
}

// captureSpan prefers the index window and falls back to the widest step window,
// so a producer that timestamped steps but not the index still gets a strip.
func captureSpan(index capture.Index) (start, end time.Time, ok bool) {
	start, startOK := parseTime(index.StartedAt)
	end, endOK := parseTime(index.EndedAt)
	for _, step := range index.Steps {
		if from, fromOK := parseTime(step.StartedAt); fromOK && (!startOK || from.Before(start)) {
			start, startOK = from, true
		}
		if to, toOK := parseTime(step.EndedAt); toOK && (!endOK || to.After(end)) {
			end, endOK = to, true
		}
	}
	if !startOK || !endOK || !end.After(start) {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

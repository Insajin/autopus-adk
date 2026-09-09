package cli

import (
	"encoding/json"
	"time"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
)

const (
	pipelineOMPActiveProbeMaxTurnUsages    = 64
	pipelineOMPActiveProbeMaxIdentifiers   = 32
	pipelineOMPActiveProbeMaxHistogramKeys = 64
	pipelineOMPActiveProbeMaxElapsedMS     = 86_400_000
)

// pipelineOMPActiveProbe collects the REQ-PROBE-001 measurement metadata that
// the managed RPC path already parses. Every hook is nil-safe and a production
// run leaves the field nil, so admission decisions are identical with and
// without a probe. The probe approves nothing, retains no provider text, and
// keeps only numbers, booleans and metadata identifiers.
type pipelineOMPActiveProbe struct {
	recorder *ompprobe.Recorder
	// failure is the first record write that did not land. A probe that lost a
	// record is not a completed probe, so the run reports that instead of
	// probe_completed.
	failure error
	// Identity of the call in flight, set by the observe-session run before it
	// hands the prompt to a session.
	sequence        int
	variant         string
	sessionSequence int
	sessionSegment  int
	// callOpen marks a call whose record has not been emitted yet, so an abort
	// attaches its reason to that call instead of filing a second record.
	callOpen    bool
	statsBefore *ompprobe.Usage
	statsAfter  *ompprobe.Usage
	turnUsages  []ompprobe.Usage
	rejected    int
	attempt     *pipelineOMPActiveProbeAttempt
}

// pipelineOMPActiveProbeTally counts metadata identifiers verbatim and
// case-sensitively. A token outside ompprobe.ValidIdentifier, or past the
// record bound, is rejected rather than folded into a placeholder key: a probe
// histogram must never invent an allowlist entry that T1 would then read back
// as observed.
type pipelineOMPActiveProbeTally struct {
	counts   map[string]int
	order    []string
	rejected int
}

func (tally *pipelineOMPActiveProbeTally) observe(name string, limit int) {
	if !ompprobe.ValidIdentifier(name) {
		tally.rejected++
		return
	}
	if _, seen := tally.counts[name]; !seen {
		if len(tally.order) >= limit {
			tally.rejected++
			return
		}
		if tally.counts == nil {
			tally.counts = make(map[string]int, limit)
		}
		tally.order = append(tally.order, name)
	}
	tally.counts[name]++
}

func (tally *pipelineOMPActiveProbeTally) histogram() map[string]int {
	if len(tally.counts) == 0 {
		return nil
	}
	return tally.counts
}

func (tally *pipelineOMPActiveProbeTally) names() []string {
	if len(tally.order) == 0 {
		return nil
	}
	return tally.order
}

func (probe *pipelineOMPActiveProbe) enabled() bool {
	return probe != nil && probe.recorder != nil
}

// beginCall binds the records that follow to one cohort call. The observe
// session owns the identity because only it knows the segment and the position
// of the call inside the reused session.
func (probe *pipelineOMPActiveProbe) beginCall(sequence int, variant string, sessionSequence, segment int) {
	if probe == nil {
		return
	}
	probe.sequence, probe.variant = sequence, variant
	probe.sessionSequence, probe.sessionSegment = sessionSequence, segment
	probe.statsBefore, probe.statsAfter = nil, nil
	probe.turnUsages, probe.rejected, probe.attempt = nil, 0, nil
	probe.callOpen = true
}

func (probe *pipelineOMPActiveProbe) observeStatsBefore(usage pipelineOMPActiveUsage, err error) {
	if probe == nil || err != nil {
		return
	}
	observed := pipelineOMPActiveProbeUsage(usage)
	probe.statsBefore = &observed
}

func (probe *pipelineOMPActiveProbe) observeStatsAfter(usage pipelineOMPActiveUsage, err error) {
	if probe == nil || err != nil {
		return
	}
	observed := pipelineOMPActiveProbeUsage(usage)
	probe.statsAfter = &observed
}

// observeTurnFrame keeps the usage a primary turn reported. The frame text is
// never read: only the numeric members of its usage object are.
func (probe *pipelineOMPActiveProbe) observeTurnFrame(frame pipelineOMPRPCFrame) {
	if probe == nil || len(probe.turnUsages) >= pipelineOMPActiveProbeMaxTurnUsages {
		return
	}
	usage, found := pipelineOMPActiveProbeFrameUsage(frame)
	if !found {
		return
	}
	probe.turnUsages = append(probe.turnUsages, usage)
}

// finishCall emits the call record. A failed call keeps whatever it observed
// plus a body-free abort token, because a partial observation is the only
// honest record of an interrupted probe.
func (probe *pipelineOMPActiveProbe) finishCall(elapsed time.Duration, abortReason string) {
	if probe == nil || !probe.callOpen {
		return
	}
	probe.callOpen = false
	if !probe.enabled() {
		return
	}
	probe.record(ompprobe.Record{
		Kind: ompprobe.KindCall, Sequence: probe.sequence, Variant: probe.variant,
		SessionSequence: probe.sessionSequence, SessionSegment: probe.sessionSegment,
		StatsBefore: probe.statsBefore, StatsAfter: probe.statsAfter,
		TurnUsages:          probe.turnUsages,
		BranchUsageDelta:    probe.branchUsageDelta(),
		UsageIdentityDelta:  probe.usageIdentityDelta(),
		ElapsedMS:           pipelineOMPActiveProbeElapsedMS(elapsed),
		AbortReason:         abortReason,
		RejectedIdentifiers: probe.rejected,
	})
}

// recordAbort names a failure that happened outside any call — handshake,
// setup, shutdown, cohort cardinality or cleanup. It carries no variant and no
// sequence because none was in flight.
func (probe *pipelineOMPActiveProbe) recordAbort(reason string) {
	if !probe.enabled() || reason == "" {
		return
	}
	probe.record(ompprobe.Record{Kind: ompprobe.KindCall, AbortReason: reason})
}

func (probe *pipelineOMPActiveProbe) record(record ompprobe.Record) {
	if err := probe.recorder.Record(record); err != nil && probe.failure == nil {
		probe.failure = err
	}
}

// branchUsageDelta is what this one call added to its own session's billed
// totals: the component-wise difference of the two get_session_stats reads
// around the primary prompt. It is signed because a compaction rewrites the
// active history and can lower a component.
func (probe *pipelineOMPActiveProbe) branchUsageDelta() *ompprobe.Usage {
	if probe.statsBefore == nil || probe.statsAfter == nil {
		return nil
	}
	before, after := probe.statsBefore, probe.statsAfter
	return &ompprobe.Usage{
		Input: after.Input - before.Input, Output: after.Output - before.Output,
		CacheRead: after.CacheRead - before.CacheRead, CacheWrite: after.CacheWrite - before.CacheWrite,
		Total: after.Total - before.Total,
	}
}

// usageIdentityDelta is the billed input the session stats moved minus the
// billed input the turn frames reported for the same prompt. Zero means the
// two observation sources agree; anything else is the diagnostic that says
// frame usage does not account for the charge. It is recorded, never gated:
// with no turn usage observed it degrades to the whole stats delta.
func (probe *pipelineOMPActiveProbe) usageIdentityDelta() int64 {
	if probe.statsBefore == nil || probe.statsAfter == nil {
		return 0
	}
	delta := pipelineOMPActiveProbeBilledInput(*probe.statsAfter) -
		pipelineOMPActiveProbeBilledInput(*probe.statsBefore)
	for _, usage := range probe.turnUsages {
		delta -= pipelineOMPActiveProbeBilledInput(usage)
	}
	return delta
}

func pipelineOMPActiveProbeBilledInput(usage ompprobe.Usage) int64 {
	return usage.Input + usage.CacheRead + usage.CacheWrite
}

func pipelineOMPActiveProbeUsage(usage pipelineOMPActiveUsage) ompprobe.Usage {
	return ompprobe.Usage{
		Input: usage.ReportedInput, Output: usage.Output,
		CacheRead: usage.CacheRead, CacheWrite: usage.CacheWrite, Total: usage.Total,
	}
}

// pipelineOMPActiveProbeFrameUsage reads the usage a lifecycle frame carries.
// OMP forwards session events verbatim, so the usage object sits either on the
// frame or on the message it wraps; both are tried and neither is required.
func pipelineOMPActiveProbeFrameUsage(frame pipelineOMPRPCFrame) (ompprobe.Usage, bool) {
	if usage, found := pipelineOMPActiveProbeDecodeUsage(frame.Usage); found {
		return usage, true
	}
	var carrier struct {
		Usage json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(frame.Message, &carrier) != nil {
		return ompprobe.Usage{}, false
	}
	return pipelineOMPActiveProbeDecodeUsage(carrier.Usage)
}

// pipelineOMPActiveProbeDecodeUsage keeps only the members that were actually
// reported. An absent member stays zero instead of being reconstructed, and a
// negative member voids the whole reading rather than being clamped.
func pipelineOMPActiveProbeDecodeUsage(raw json.RawMessage) (ompprobe.Usage, bool) {
	if len(raw) == 0 {
		return ompprobe.Usage{}, false
	}
	var reported struct {
		Input      *int64 `json:"input"`
		Output     *int64 `json:"output"`
		CacheRead  *int64 `json:"cacheRead"`
		CacheWrite *int64 `json:"cacheWrite"`
		Total      *int64 `json:"totalTokens"`
	}
	if json.Unmarshal(raw, &reported) != nil {
		return ompprobe.Usage{}, false
	}
	usage := ompprobe.Usage{}
	found := false
	for _, member := range []struct {
		value **int64
		field *int64
	}{
		{&reported.Input, &usage.Input}, {&reported.Output, &usage.Output},
		{&reported.CacheRead, &usage.CacheRead}, {&reported.CacheWrite, &usage.CacheWrite},
		{&reported.Total, &usage.Total},
	} {
		if *member.value == nil {
			continue
		}
		if **member.value < 0 {
			return ompprobe.Usage{}, false
		}
		*member.field, found = **member.value, true
	}
	return usage, found
}

func pipelineOMPActiveProbeElapsedMS(elapsed time.Duration) int64 {
	milliseconds := elapsed.Milliseconds()
	if milliseconds < 0 {
		return 0
	}
	if milliseconds > pipelineOMPActiveProbeMaxElapsedMS {
		return pipelineOMPActiveProbeMaxElapsedMS
	}
	return milliseconds
}

package ompprobe

// The probe cohort is a fixed schedule, not a record count (SPEC-OMP-007
// REQ-PROBE-001): 20 task pairs run serially across two 10-pair session
// segments, every pair runs the same task once on the full-history variant and
// once on the optimized variant, the two orders are balanced 10/10, and every
// optimized call that reuses an already-open session is preceded by exactly one
// compaction attempt. That is 40 call records and 18 compaction records.
//
// Counting records cannot establish it. A truncated run, a repeated sequence
// or a shuffled schedule can all reach the same totals, so completeness is
// decided per scheduled position from what each retained record says about
// itself.
const (
	cohortSegments     = 2
	cohortSegmentPairs = 10
	cohortPairs        = cohortSegments * cohortSegmentPairs
	cohortCalls        = cohortPairs * 2
	// cohortPairOrders is how many pairs must run each of the two orders.
	cohortPairOrders = cohortPairs / 2
	// cohortAttempts is one attempt before every optimized call except the one
	// that opens its session.
	cohortAttempts = cohortSegments * (cohortSegmentPairs - 1)
)

// The whole schedule has to fit the writer's bounds, or a complete probe could
// never be retained. Compiles only while MaxRecords leaves room for it.
const _ = uint(MaxRecords - (cohortCalls + cohortAttempts))

// cohortCoverage is what a set of retained records establishes about the
// schedule. The counts are per kind over exactly those records; complete
// reports that they are the schedule itself.
type cohortCoverage struct {
	callRecords       int
	compactionRecords int
	complete          bool
}

// cohortSlot is one scheduled call as its retained records describe it.
type cohortSlot struct {
	present         bool
	aborted         bool
	variant         string
	sessionSequence int
	sessionSegment  int
	attempts        int
}

// cohortLedger maps records onto the schedule by call sequence.
type cohortLedger struct {
	slots [cohortCalls]cohortSlot
	// stray marks a record that cannot occupy any scheduled position: an
	// out-of-schedule sequence, a repeated call, a stage-level abort record, or
	// an attempt filed under a variant the schedule never compacts. Totals hide
	// those, so one stray record forfeits completeness on its own.
	stray bool
}

// assessCohort reads the records once and reports what they cover. Callers
// pass only records they accepted: a rejected set publishes nothing, so it
// never claims a complete cohort.
func assessCohort(records []Record) cohortCoverage {
	var coverage cohortCoverage
	var ledger cohortLedger
	for index := range records {
		record := &records[index]
		switch record.Kind {
		case KindCall:
			coverage.callRecords++
			ledger.admitCall(record)
		case KindCompaction:
			coverage.compactionRecords++
			ledger.admitAttempt(record)
		}
	}
	coverage.complete = ledger.complete()
	return coverage
}

// slot resolves the scheduled position a record claims. The stage-level abort
// record carries no variant and no sequence, so it resolves to nothing: it is
// an interrupted probe explaining itself, never a scheduled observation.
func (ledger *cohortLedger) slot(sequence int, variant string) *cohortSlot {
	if variant != VariantFull && variant != VariantOptimized {
		return nil
	}
	if sequence < 1 || sequence > cohortCalls {
		return nil
	}
	return &ledger.slots[sequence-1]
}

func (ledger *cohortLedger) admitCall(record *Record) {
	slot := ledger.slot(record.Sequence, record.Variant)
	if slot == nil || slot.present || record.StatsBefore == nil || record.StatsAfter == nil {
		ledger.stray = true
		return
	}
	slot.present = true
	slot.variant = record.Variant
	slot.sessionSequence, slot.sessionSegment = record.SessionSequence, record.SessionSegment
	if record.AbortReason != "" {
		slot.aborted = true
	}
}

// admitAttempt files a compaction attempt against the call it ran before. Its
// own variant is checked here; whether that agrees with the call record
// occupying the position is decided per pair. An attempt that died mid-flight
// interrupted the call it belongs to, so it lands on the slot rather than
// being counted as a clean one.
func (ledger *cohortLedger) admitAttempt(record *Record) {
	slot := ledger.slot(record.Sequence, record.Variant)
	if slot == nil || record.Variant != VariantOptimized {
		ledger.stray = true
		return
	}
	slot.attempts++
	if record.AbortReason != "" {
		slot.aborted = true
	}
}

func (ledger *cohortLedger) complete() bool {
	if ledger.stray {
		return false
	}
	fullFirst := 0
	for pair := range cohortPairs {
		first, second := &ledger.slots[pair*2], &ledger.slots[pair*2+1]
		if !scheduledPair(pair, first, second) {
			return false
		}
		if first.variant == VariantFull {
			fullFirst++
		}
	}
	// Both orders were exercised equally, which is what makes the pairs
	// comparable in the first place.
	return fullFirst == cohortPairOrders
}

// scheduledPair decides one task pair. Both calls must have landed uninterrupted,
// the pair must hold one call of each variant, both must name the session
// position the schedule gives that pair, and the optimized call must carry
// exactly the one attempt made before it — or none when it opens its session.
// Outcome, method and attempt coverage are not read: a probe that only ever
// declined to compact still observed the whole schedule.
func scheduledPair(pair int, first, second *cohortSlot) bool {
	if !first.present || !second.present || first.aborted || second.aborted {
		return false
	}
	full, optimized := first, second
	if first.variant != VariantFull {
		full, optimized = second, first
	}
	if full.variant != VariantFull || optimized.variant != VariantOptimized {
		return false
	}
	segment, sessionSequence := pair/cohortSegmentPairs+1, pair%cohortSegmentPairs+1
	if full.sessionSegment != segment || optimized.sessionSegment != segment ||
		full.sessionSequence != sessionSequence || optimized.sessionSequence != sessionSequence {
		return false
	}
	attempts := 0
	if sessionSequence > 1 {
		attempts = 1
	}
	return full.attempts == 0 && optimized.attempts == attempts
}

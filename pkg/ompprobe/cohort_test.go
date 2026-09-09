package ompprobe

import "testing"

// cohortSchedule builds the records a probe run leaves behind: task pairs in
// session segments of ten, each pair running the same task once per variant,
// with one compaction attempt filed before every optimized call that reuses an
// already-open session. optimizedFirst decides one pair's order, which is how
// the AB/BA balance of the cohort is set.
func cohortSchedule(pairs int, optimizedFirst func(pair int) bool) []Record {
	records := make([]Record, 0, pairs*3)
	for pair := range pairs {
		segment, sessionSequence := pair/cohortSegmentPairs+1, pair%cohortSegmentPairs+1
		variants := [2]string{VariantFull, VariantOptimized}
		if optimizedFirst(pair) {
			variants = [2]string{VariantOptimized, VariantFull}
		}
		for position, variant := range variants {
			sequence := pair*2 + position + 1
			if variant == VariantOptimized && sessionSequence > 1 {
				records = append(records, cohortAttemptRecord(sequence, sessionSequence, segment))
			}
			records = append(records, cohortCallRecord(sequence, variant, sessionSequence, segment))
		}
	}
	return records
}

// cohortScheduleRecords is the whole balanced cohort: 40 call records and 18
// compaction attempts (SPEC-OMP-007 REQ-PROBE-001, AC-010).
func cohortScheduleRecords() []Record {
	return cohortSchedule(cohortPairs, func(pair int) bool { return pair%2 == 1 })
}

func cohortCallRecord(sequence int, variant string, sessionSequence, segment int) Record {
	record := callFixture(sequence)
	record.Variant = variant
	record.SessionSequence, record.SessionSegment = sessionSequence, segment
	return record
}

// cohortAttemptRecord is an attempt filed under the optimized variant, which
// is the only variant the schedule ever compacts.
func cohortAttemptRecord(sequence, sessionSequence, segment int) Record {
	record := compactionFixture(sequence)
	record.SessionSequence, record.SessionSegment = sessionSequence, segment
	return record
}

func cohortFullVariantAttempt(sequence, sessionSequence, segment int) Record {
	record := cohortAttemptRecord(sequence, sessionSequence, segment)
	record.Variant = VariantFull
	return record
}

// mutateCohortRecord rewrites the single record of kind carrying sequence.
func mutateCohortRecord(t *testing.T, records []Record, kind string, sequence int, mutate func(*Record)) []Record {
	t.Helper()
	for index := range records {
		if records[index].Kind == kind && records[index].Sequence == sequence {
			mutate(&records[index])
			return records
		}
	}
	t.Fatalf("schedule holds no %s record for sequence %d", kind, sequence)
	return nil
}

func dropCohortRecord(t *testing.T, records []Record, kind string, sequence int) []Record {
	t.Helper()
	for index := range records {
		if records[index].Kind == kind && records[index].Sequence == sequence {
			return append(records[:index:index], records[index+1:]...)
		}
	}
	t.Fatalf("schedule holds no %s record for sequence %d", kind, sequence)
	return nil
}

func TestAssessCohort_AcceptsTheScheduledCohort(t *testing.T) {
	coverage := assessCohort(cohortScheduleRecords())
	if coverage.callRecords != 40 || coverage.compactionRecords != 18 {
		t.Fatalf("coverage counts %d calls and %d attempts, want 40/18",
			coverage.callRecords, coverage.compactionRecords)
	}
	if !coverage.complete {
		t.Fatal("the whole probe schedule was not reported complete")
	}
}

// A probe that only ever declined to compact still observed the whole
// schedule: the retained file establishes coverage, not benefit, so no
// successful compaction is required and unknown attempt coverage stands.
func TestAssessCohort_AcceptsAScheduleThatOnlyDeclinedToCompact(t *testing.T) {
	records := cohortScheduleRecords()
	declines := 0
	for index := range records {
		if records[index].Kind != KindCompaction {
			continue
		}
		records[index] = Record{
			Kind: KindCompaction, Sequence: records[index].Sequence, Variant: records[index].Variant,
			SessionSequence: records[index].SessionSequence,
			SessionSegment:  records[index].SessionSegment,
			ElapsedMS:       12, Outcome: OutcomeRefused, Method: MethodNone,
			Refusal: RefusalTooSmall, AttemptCoverage: AttemptCoverageUnknown,
		}
		if err := Validate(records[index]); err != nil {
			t.Fatalf("declined attempt is outside the schema: %v", err)
		}
		declines++
	}
	if declines != 18 {
		t.Fatalf("rewrote %d attempts, want 18", declines)
	}
	if coverage := assessCohort(records); !coverage.complete {
		t.Fatal("a decline-only cohort was not reported complete")
	}
}

func TestAssessCohort_RefusesRecordsThatAreNotTheSchedule(t *testing.T) {
	alternating := func(pair int) bool { return pair%2 == 1 }
	tests := []struct {
		name        string
		records     []Record
		calls       int
		compactions int
	}{
		{
			name:        "the run stopped after twelve pairs",
			records:     cohortSchedule(12, alternating),
			calls:       24,
			compactions: 10,
		},
		{
			name: "one call repeats another call's sequence",
			records: mutateCohortRecord(t, cohortScheduleRecords(), KindCall, 40,
				func(record *Record) { record.Sequence = 1 }),
			calls:       40,
			compactions: 18,
		},
		{
			name:        "one reused call has no compaction attempt",
			records:     dropCohortRecord(t, cohortScheduleRecords(), KindCompaction, 3),
			calls:       40,
			compactions: 17,
		},
		{
			name:        "one call carries two compaction attempts",
			records:     append(cohortScheduleRecords(), cohortAttemptRecord(3, 2, 1)),
			calls:       40,
			compactions: 19,
		},
		{
			name:        "an attempt is filed against the full-history variant",
			records:     append(cohortScheduleRecords(), cohortFullVariantAttempt(4, 2, 1)),
			calls:       40,
			compactions: 19,
		},
		{
			name:        "an attempt precedes the call that opens its session",
			records:     append(cohortScheduleRecords(), cohortAttemptRecord(2, 1, 1)),
			calls:       40,
			compactions: 19,
		},
		{
			name: "one call disagrees with its session position",
			records: mutateCohortRecord(t, cohortScheduleRecords(), KindCall, 1,
				func(record *Record) { record.SessionSequence = 3 }),
			calls:       40,
			compactions: 18,
		},
		{
			name: "one call disagrees with its segment",
			records: mutateCohortRecord(t, cohortScheduleRecords(), KindCall, 21,
				func(record *Record) { record.SessionSegment = 1 }),
			calls:       40,
			compactions: 18,
		},
		{
			name:        "every pair ran in the same order",
			records:     cohortSchedule(cohortPairs, func(int) bool { return false }),
			calls:       40,
			compactions: 18,
		},
		{
			name: "one call was interrupted",
			records: mutateCohortRecord(t, cohortScheduleRecords(), KindCall, 7,
				func(record *Record) { record.AbortReason = "runtime_readback_failed" }),
			calls:       40,
			compactions: 18,
		},
		{
			name: "one compaction attempt was interrupted",
			records: mutateCohortRecord(t, cohortScheduleRecords(), KindCompaction, 3,
				func(record *Record) {
					record.Outcome, record.AbortReason = OutcomeFailed, "compaction_barrier_crossed"
				}),
			calls:       40,
			compactions: 18,
		},
		{
			name: "the probe died outside any call",
			records: append(cohortScheduleRecords(),
				Record{Kind: KindCall, AbortReason: "canary_stopped"}),
			calls:       41,
			compactions: 18,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, record := range test.records {
				if err := Validate(record); err != nil {
					t.Fatalf("mutated record is outside the schema: %v", err)
				}
			}
			coverage := assessCohort(test.records)
			if coverage.complete {
				t.Fatal("an unscheduled record set claimed a complete cohort")
			}
			if coverage.callRecords != test.calls || coverage.compactionRecords != test.compactions {
				t.Fatalf("coverage counts %d calls and %d attempts, want %d/%d",
					coverage.callRecords, coverage.compactionRecords, test.calls, test.compactions)
			}
		})
	}
}

func TestParseRecordSet_ReportsCoverageOfTheRecordsItRetains(t *testing.T) {
	body, err := MarshalRecordSet(cohortScheduleRecords())
	if err != nil {
		t.Fatalf("marshal the cohort schedule: %v", err)
	}
	set, err := ParseRecordSet(body)
	if err != nil {
		t.Fatalf("parse the cohort schedule: %v", err)
	}
	if set.Accepted != 58 || set.Rejected != 0 || !set.Complete {
		t.Fatalf("set is accepted=%d rejected=%d complete=%v, want 58/0/true",
			set.Accepted, set.Rejected, set.Complete)
	}
	if set.CallRecords != 40 || set.CompactionRecords != 18 {
		t.Fatalf("set holds %d calls and %d attempts, want 40/18",
			set.CallRecords, set.CompactionRecords)
	}
	if set.CallRecords+set.CompactionRecords != set.Accepted {
		t.Fatal("the per-kind counts do not account for every accepted record")
	}

	// A set with any rejection publishes nothing, so it never claims a
	// complete cohort no matter how much of the schedule it did hold.
	rejected, err := ParseRecordSet(append(body, []byte("{\"kind\":\"call\",\"sequence\":99999}\n")...))
	if err == nil {
		t.Fatal("an out-of-range record was accepted")
	}
	if rejected.Rejected != 1 || rejected.Accepted != 58 {
		t.Fatalf("set is accepted=%d rejected=%d, want 58/1", rejected.Accepted, rejected.Rejected)
	}
	if rejected.Complete {
		t.Fatal("a rejected record set claimed a complete cohort")
	}
	if rejected.CallRecords != 40 || rejected.CompactionRecords != 18 {
		t.Fatalf("set holds %d calls and %d attempts, want 40/18",
			rejected.CallRecords, rejected.CompactionRecords)
	}
}

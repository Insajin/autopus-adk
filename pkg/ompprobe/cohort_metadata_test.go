package ompprobe

import "testing"

func TestAssessCohort_MissingCallStatsIsIncomplete(t *testing.T) {
	for _, before := range []bool{true, false} {
		records := cohortScheduleRecords()
		records = mutateCohortRecord(t, records, KindCall, 1, func(record *Record) {
			if before {
				record.StatsBefore = nil
			} else {
				record.StatsAfter = nil
			}
		})
		if assessCohort(records).complete {
			t.Fatal("complete schedule with missing call measurements claimed completeness")
		}
	}
}

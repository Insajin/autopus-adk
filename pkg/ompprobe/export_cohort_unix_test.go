//go:build darwin || linux

package ompprobe

import (
	"bytes"
	"os"
	"testing"
)

// The lane must be able to tell a whole retained cohort from a probe that
// merely wrote records, and a partial cohort is still worth retaining: the
// staging and the final export are the same call, so both report coverage the
// same way and neither refuses records only because the run stopped early.
func TestExport_ReportsTheCoverageOfWhatItRetained(t *testing.T) {
	alternating := func(pair int) bool { return pair%2 == 1 }
	tests := []struct {
		name        string
		records     []Record
		calls       int
		compactions int
		complete    bool
	}{
		{
			name:        "the whole schedule",
			records:     cohortScheduleRecords(),
			calls:       40,
			compactions: 18,
			complete:    true,
		},
		{
			name:        "a run that stopped after twelve pairs",
			records:     cohortSchedule(12, alternating),
			calls:       24,
			compactions: 10,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExportFixture(t)
			body, err := MarshalRecordSet(test.records)
			if err != nil {
				t.Fatalf("marshal the schedule: %v", err)
			}
			writeFixtureFile(t, fixture.sourcePath, body)
			summary, err := Export(fixture.options)
			if err != nil {
				t.Fatalf("export the schedule: %v", err)
			}
			if summary.CallRecords != test.calls || summary.CompactionRecords != test.compactions {
				t.Fatalf("summary holds %d calls and %d attempts, want %d/%d",
					summary.CallRecords, summary.CompactionRecords, test.calls, test.compactions)
			}
			if summary.Accepted != test.calls+test.compactions || summary.Rejected != 0 {
				t.Fatalf("summary is accepted=%d rejected=%d, want %d/0",
					summary.Accepted, summary.Rejected, test.calls+test.compactions)
			}
			if summary.Complete != test.complete {
				t.Fatalf("summary reports complete=%v, want %v", summary.Complete, test.complete)
			}
			published, err := os.ReadFile(fixture.destination)
			if err != nil {
				t.Fatalf("read retained file: %v", err)
			}
			if !bytes.Equal(published, body) {
				t.Fatal("retained bytes are not the re-serialized schedule")
			}
		})
	}
}

// A refused export retains nothing, so it accounts for the records it read and
// claims no cohort — even when the records it did read were the whole schedule.
func TestExport_NeverClaimsACohortItDidNotRetain(t *testing.T) {
	fixture := newExportFixture(t)
	body, err := MarshalRecordSet(cohortScheduleRecords())
	if err != nil {
		t.Fatalf("marshal the schedule: %v", err)
	}
	writeFixtureFile(t, fixture.sourcePath, append(body, []byte("{\"kind\":\"call\",\"sequence\":99999}\n")...))
	summary, err := Export(fixture.options)
	if err == nil {
		t.Fatal("an out-of-range record was exported")
	}
	if summary.Accepted != 58 || summary.Rejected != 1 {
		t.Fatalf("summary is accepted=%d rejected=%d, want 58/1", summary.Accepted, summary.Rejected)
	}
	if summary.CallRecords != 40 || summary.CompactionRecords != 18 {
		t.Fatalf("summary holds %d calls and %d attempts, want 40/18",
			summary.CallRecords, summary.CompactionRecords)
	}
	if summary.Complete {
		t.Fatal("a refused export claimed a complete cohort")
	}
	assertNothingPublished(t, fixture)
}

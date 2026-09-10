package ompprobe

import "testing"

func TestCheckpointCountsAreCompactionMetadata(t *testing.T) {
	record := compactionFixture(6)
	record.PreCheckpoints, record.PostCheckpoints = 2, 1
	body, err := MarshalRecordSet([]Record{record})
	if err != nil {
		t.Fatal(err)
	}
	set, err := ParseRecordSet(body)
	if err != nil {
		t.Fatal(err)
	}
	if set.Records[0].PreCheckpoints != 2 || set.Records[0].PostCheckpoints != 1 {
		t.Fatal("checkpoint observations were lost")
	}
	call := callFixture(6)
	call.PreCheckpoints = 1
	if Validate(call) != errRecordScope {
		t.Fatal("checkpoint count accepted on primary call")
	}
}

func TestCheckpointCountsRejectInvalidBounds(t *testing.T) {
	for _, count := range []int{-1, maxCountValue + 1} {
		record := compactionFixture(6)
		record.PreCheckpoints = count
		if Validate(record) != errRecordCount {
			t.Fatal("invalid pre-checkpoint count accepted")
		}
		record.PreCheckpoints = 0
		record.PostCheckpoints = count
		if Validate(record) != errRecordCount {
			t.Fatal("invalid post-checkpoint count accepted")
		}
	}
}

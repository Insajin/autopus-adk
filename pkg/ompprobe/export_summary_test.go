package ompprobe

import (
	"encoding/json"
	"testing"
)

// The release lane reads these keys with jq on both the publish and the refusal
// path, so every field has to be on the wire even at its zero value. An
// omitempty on Complete would turn "this cohort is not complete" into "there is
// no such field", and a lane that cannot read it falls back to counting
// records — which is exactly the judgement the field exists to replace.
func TestExportSummary_KeepsEveryFieldOnTheWire(t *testing.T) {
	encoded, err := json.Marshal(ExportSummary{})
	if err != nil {
		t.Fatalf("marshal an empty summary: %v", err)
	}
	const want = `{"accepted":0,"rejected":0,"call_records":0,"compaction_records":0,"complete":false}`
	if string(encoded) != want {
		t.Fatalf("summary wire form is %s, want %s", encoded, want)
	}
}

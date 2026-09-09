package ompprobe

import (
	"bytes"
	"reflect"
	"testing"
)

func TestParseRecordSet_RoundTripsValidatedRecords(t *testing.T) {
	body := fixtureBody(t)
	set, err := ParseRecordSet(body)
	if err != nil {
		t.Fatalf("parse valid record file: %v", err)
	}
	if set.Accepted != 2 || set.Rejected != 0 {
		t.Fatalf("counts are accepted=%d rejected=%d, want 2/0", set.Accepted, set.Rejected)
	}
	want := []Record{callFixture(1), compactionFixture(2)}
	if !reflect.DeepEqual(set.Records, want) {
		t.Fatal("decoded records differ from the written records")
	}
	republished, err := MarshalRecordSet(set.Records)
	if err != nil {
		t.Fatalf("re-serialize records: %v", err)
	}
	if !bytes.Equal(republished, body) {
		t.Fatal("re-serialized records are not byte-identical to the validated input")
	}
}

func TestParseRecordSet_RejectsUnknownFieldsAndSmuggledBodies(t *testing.T) {
	line := fixtureLine(t, callFixture(1))
	for _, injection := range []string{
		`"prompt":"you are a helpful assistant"`,
		`"credential":"sk-live-000"`,
		`"assistant_text":"hello"`,
		`"Kind":"call"`,
	} {
		body := spliceRecordField(line, injection)
		set, err := ParseRecordSet(body)
		if err == nil {
			t.Fatalf("record carrying %s accepted", injection)
		}
		if set.Records != nil || set.Rejected != 1 || set.Accepted != 0 {
			t.Fatalf("rejected record leaked into the set: %+v", set)
		}
	}
}

func TestParseRecordSet_RejectsDuplicateKeysAndTrailingValues(t *testing.T) {
	line := fixtureLine(t, callFixture(1))
	duplicate := spliceRecordField(line, `"kind":"call"`)
	if _, err := ParseRecordSet(duplicate); err != errRecordDuplicateKey {
		t.Fatalf("duplicate key error is %v, want %v", err, errRecordDuplicateKey)
	}
	trailing := append(append([]byte(nil), line...), []byte("{}\n")...)
	if _, err := ParseRecordSet(trailing); err != errRecordTrailing {
		t.Fatalf("trailing JSON error is %v, want %v", err, errRecordTrailing)
	}
}

func TestParseRecordSet_RejectsUnframedAndUndecodableFiles(t *testing.T) {
	line := fixtureLine(t, callFixture(1))
	tests := []struct {
		name string
		body []byte
		want error
	}{
		{name: "empty", body: nil, want: errRecordSetSize},
		{name: "no trailing newline", body: line, want: errRecordSetFraming},
		{
			name: "invalid utf8",
			body: append(append([]byte(nil), []byte{'{', '"', 0xff, '"', ':', '1', '}'}...), '\n'),
			want: errRecordSetEncoding,
		},
		{
			name: "blank line",
			body: append(append(append([]byte(nil), line...), '\n', '\n'), append(append([]byte(nil), line...), '\n')...),
			want: errRecordLine,
		},
		{
			name: "carriage return",
			body: append(append([]byte(nil), line...), '\r', '\n'),
			want: errRecordLine,
		},
		{name: "oversized file", body: bytes.Repeat([]byte("z"), MaxTotalBytes+1), want: errRecordSetSize},
		{
			name: "oversized record",
			body: append(append(append([]byte("{"), bytes.Repeat([]byte(" "), MaxRecordBytes)...), line[1:]...), '\n'),
			want: errRecordLine,
		},
		{name: "too many records", body: bytes.Repeat(append(append([]byte(nil), line...), '\n'), MaxRecords+1), want: errRecordSetCardinality},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseRecordSet(test.body); err != test.want {
				t.Fatalf("error is %v, want %v", err, test.want)
			}
		})
	}
}

func TestParseRecordSet_RejectsOutOfRangeRecords(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Record)
		want   error
	}{
		{name: "unknown kind", mutate: func(r *Record) { r.Kind = "transcript" }, want: errRecordKind},
		{name: "unknown variant", mutate: func(r *Record) { r.Variant = "C" }, want: errRecordIdentity},
		{name: "negative sequence", mutate: func(r *Record) { r.Sequence = -1 }, want: errRecordIdentity},
		{name: "sequence past bound", mutate: func(r *Record) { r.Sequence = maxSequenceValue + 1 }, want: errRecordIdentity},
		{name: "negative elapsed", mutate: func(r *Record) { r.ElapsedMS = -1 }, want: errRecordIdentity},
		{
			name:   "negative undeclared usage",
			mutate: func(r *Record) { r.StatsBefore = &Usage{Input: -1} },
			want:   errRecordUsage,
		},
		{
			name:   "usage past bound",
			mutate: func(r *Record) { r.StatsAfter = &Usage{Total: maxTokenValue + 1} },
			want:   errRecordUsage,
		},
		{
			name:   "turn usages past bound",
			mutate: func(r *Record) { r.TurnUsages = make([]Usage, maxTurnUsages+1) },
			want:   errRecordCardinality,
		},
		{
			name:   "compaction enum on a call record",
			mutate: func(r *Record) { r.Method = MethodRemote },
			want:   errRecordScope,
		},
		{
			name:   "identifier outside the range",
			mutate: func(r *Record) { r.Roles = map[string]int{"Opaque-Item!": 1} },
			want:   errRecordIdentifier,
		},
		{
			name:   "count past bound",
			mutate: func(r *Record) { r.ContentTypes = map[string]int{"text": maxCountValue + 1} },
			want:   errRecordCount,
		},
		{
			name:   "identifier list past bound",
			mutate: func(r *Record) { r.UIRequestMethods = make([]string, maxIdentifierList+1) },
			want:   errRecordCardinality,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := callFixture(1)
			test.mutate(&record)
			if err := Validate(record); err != test.want {
				t.Fatalf("validation error is %v, want %v", err, test.want)
			}
		})
	}
}

func TestValidate_ScopesCompactionEnumsAndTheStageAbortRecord(t *testing.T) {
	compaction := compactionFixture(2)
	for _, blank := range []func(*Record){
		func(r *Record) { r.Outcome = "" },
		func(r *Record) { r.Method = "" },
		func(r *Record) { r.Refusal = "" },
		func(r *Record) { r.AttemptCoverage = "" },
	} {
		record := compaction
		blank(&record)
		if err := Validate(record); err != errRecordEnum {
			t.Fatalf("compaction record with a missing enum returned %v", err)
		}
	}
	abort := Record{Kind: KindCall, AbortReason: "probe_interrupted"}
	if err := Validate(abort); err != nil {
		t.Fatalf("stage-level abort record rejected: %v", err)
	}
	if err := Validate(Record{Kind: KindCall}); err != errRecordIdentity {
		t.Fatal("a variant-less record without an abort reason must be rejected")
	}
	sequenced := abort
	sequenced.Sequence = 3
	if err := Validate(sequenced); err != errRecordIdentity {
		t.Fatal("a variant-less record must not carry a call sequence")
	}
	worded := abort
	worded.AbortReason = "compact refused: model said no"
	if err := Validate(worded); err != errRecordEnum {
		t.Fatal("abort reasons must be body-free tokens")
	}
}

// spliceRecordField inserts a raw JSON member right after the opening brace.
func spliceRecordField(line []byte, member string) []byte {
	body := make([]byte, 0, len(line)+len(member)+2)
	body = append(body, '{')
	body = append(body, member...)
	body = append(body, ',')
	body = append(body, line[1:]...)
	return append(body, '\n')
}

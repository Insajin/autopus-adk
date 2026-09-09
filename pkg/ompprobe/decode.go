package ompprobe

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

const maxRecordDepth = 8

var (
	errRecordSetSize        = errors.New("probe record file size is invalid")
	errRecordSetEncoding    = errors.New("probe record file is not valid UTF-8")
	errRecordSetFraming     = errors.New("probe record file framing is invalid")
	errRecordSetCardinality = errors.New("probe record file holds too many records")
	errRecordLine           = errors.New("probe record is malformed")
	errRecordDuplicateKey   = errors.New("probe record contains a duplicate key")
	errRecordTrailing       = errors.New("probe record contains trailing JSON")
	errRecordUnknownField   = errors.New("probe record contains an unknown field")
)

// RecordSet is the outcome of reading a probe record file. Records is
// populated only when nothing was rejected, so a caller cannot accidentally
// publish a partially trusted set.
//
// CallRecords and CompactionRecords account for every accepted record by
// kind. Complete additionally reports that those records are the whole probe
// cohort schedule; it is false whenever anything was rejected, because a
// rejected set is never published.
type RecordSet struct {
	Records           []Record
	Accepted          int
	Rejected          int
	CallRecords       int
	CompactionRecords int
	Complete          bool
}

// ParseRecordSet strictly decodes and validates a JSONL probe record file.
// Unknown fields, duplicate keys, invalid UTF-8, trailing JSON, blank lines,
// oversized lines and out-of-range values are all rejections, and any
// rejection fails the whole set: the exporter must never publish a file it
// could not fully account for. The returned error is a fixed string that never
// quotes the input.
func ParseRecordSet(body []byte) (RecordSet, error) {
	var set RecordSet
	if len(body) == 0 || len(body) > MaxTotalBytes {
		return set, errRecordSetSize
	}
	if !utf8.Valid(body) {
		return set, errRecordSetEncoding
	}
	if body[len(body)-1] != '\n' {
		return set, errRecordSetFraming
	}
	lines := bytes.Split(body[:len(body)-1], []byte{'\n'})
	if len(lines) > MaxRecords {
		return set, errRecordSetCardinality
	}
	records := make([]Record, 0, len(lines))
	var failure error
	for _, line := range lines {
		record, err := decodeRecordLine(line)
		if err != nil {
			set.Rejected++
			if failure == nil {
				failure = err
			}
			continue
		}
		records = append(records, record)
		set.Accepted++
	}
	coverage := assessCohort(records)
	set.CallRecords, set.CompactionRecords = coverage.callRecords, coverage.compactionRecords
	if failure != nil {
		return set, failure
	}
	if set.Accepted == 0 {
		return set, errRecordSetSize
	}
	set.Records = records
	set.Complete = coverage.complete
	return set, nil
}

// MarshalRecordSet re-serializes validated records. The exporter publishes
// only these bytes, never the bytes it read.
func MarshalRecordSet(records []Record) ([]byte, error) {
	if len(records) == 0 || len(records) > MaxRecords {
		return nil, errRecordSetCardinality
	}
	buffer := bytes.NewBuffer(make([]byte, 0, 1024*len(records)))
	for _, record := range records {
		if err := Validate(record); err != nil {
			return nil, err
		}
		body, err := json.Marshal(record)
		if err != nil {
			return nil, errRecorderEncode
		}
		if len(body)+1 > MaxRecordBytes {
			return nil, errRecordLine
		}
		buffer.Write(body)
		buffer.WriteByte('\n')
		if buffer.Len() > MaxTotalBytes {
			return nil, errRecordSetSize
		}
	}
	return buffer.Bytes(), nil
}

func decodeRecordLine(line []byte) (Record, error) {
	if len(line) == 0 || len(line)+1 > MaxRecordBytes || line[len(line)-1] == '\r' {
		return Record{}, errRecordLine
	}
	if err := rejectDuplicateRecordKeys(line); err != nil {
		return Record{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, errRecordLine
	}
	if err := requireRecordEOF(decoder); err != nil {
		return Record{}, err
	}
	if err := Validate(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func rejectDuplicateRecordKeys(line []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(line))
	if err := consumeUniqueRecordValue(decoder, 0); err != nil {
		return err
	}
	return requireRecordEOF(decoder)
}

// consumeUniqueRecordValue walks the token stream because encoding/json keeps
// the last value of a duplicated key silently and matches field names
// case-insensitively, so "Kind" would otherwise pass as an alias of "kind".
// Top-level keys are therefore checked against the schema case-sensitively.
// Depth is bounded so a hostile nesting chain cannot exhaust the stack before
// the size check matters.
func consumeUniqueRecordValue(decoder *json.Decoder, depth int) error {
	if depth > maxRecordDepth {
		return errRecordLine
	}
	token, err := decoder.Token()
	if err != nil {
		return errRecordLine
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return errRecordLine
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			keyToken, keyErr := decoder.Token()
			key, ok := keyToken.(string)
			if keyErr != nil || !ok {
				return errRecordLine
			}
			if depth == 0 && !knownRecordField(key) {
				return errRecordUnknownField
			}
			if seen[key] {
				return errRecordDuplicateKey
			}
			seen[key] = true
		}
		if err := consumeUniqueRecordValue(decoder, depth+1); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	expected := json.Delim(']')
	if delimiter == '{' {
		expected = '}'
	}
	if err != nil || closing != expected {
		return errRecordLine
	}
	return nil
}

// recordFieldNames is derived from the struct tags so the accepted key set
// cannot drift from the schema.
var recordFieldNames = buildRecordFieldNames()

func buildRecordFieldNames() map[string]struct{} {
	recordType := reflect.TypeOf(Record{})
	names := make(map[string]struct{}, recordType.NumField())
	for index := range recordType.NumField() {
		tag := recordType.Field(index).Tag.Get("json")
		if separator := strings.IndexByte(tag, ','); separator >= 0 {
			tag = tag[:separator]
		}
		if tag == "" || tag == "-" {
			continue
		}
		names[tag] = struct{}{}
	}
	return names
}

func knownRecordField(key string) bool {
	_, known := recordFieldNames[key]
	return known
}

func requireRecordEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errRecordTrailing
	}
	return nil
}

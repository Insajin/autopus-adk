package ompprobe

import "errors"

// Structural bounds. They are intentionally small: a probe record describes a
// cohort call, not a transcript, so any value beyond these is a producer bug
// or a tampered file.
const (
	maxSequenceValue  = 4096
	maxSegmentValue   = 64
	maxElapsedMS      = int64(24 * 60 * 60 * 1000)
	maxTokenValue     = int64(1) << 40
	maxCountValue     = 1 << 20
	maxTurnUsages     = 64
	maxHistogramKeys  = 64
	maxIdentifierList = 32
)

// Every error below is a fixed string. Probe validation never quotes the input,
// so a malformed or hostile record cannot smuggle bytes into a log line.
var (
	errRecordKind        = errors.New("probe record kind is invalid")
	errRecordIdentity    = errors.New("probe record identity is invalid")
	errRecordUsage       = errors.New("probe record usage is out of range")
	errRecordEnum        = errors.New("probe record enum value is invalid")
	errRecordScope       = errors.New("probe record field is not valid for its kind")
	errRecordIdentifier  = errors.New("probe record identifier is out of range")
	errRecordCardinality = errors.New("probe record collection is too large")
	errRecordCount       = errors.New("probe record count is out of range")
)

// Validate reports whether record is inside the closed probe schema. It is the
// single gate used by both the writer and the exporter, so a record that a
// Recorder accepted is exactly a record the exporter will republish.
func Validate(record Record) error {
	if record.Kind != KindCall && record.Kind != KindCompaction {
		return errRecordKind
	}
	if err := validateRecordIdentity(record); err != nil {
		return err
	}
	if err := validateRecordUsage(record); err != nil {
		return err
	}
	if err := validateRecordEnums(record); err != nil {
		return err
	}
	return validateRecordMetadata(record)
}

func validateRecordIdentity(record Record) error {
	if err := validateRecordVariant(record); err != nil {
		return err
	}
	if record.Sequence < 0 || record.Sequence > maxSequenceValue ||
		record.SessionSequence < 0 || record.SessionSequence > maxSequenceValue ||
		record.SessionSegment < 0 || record.SessionSegment > maxSegmentValue {
		return errRecordIdentity
	}
	if record.ElapsedMS < 0 || record.ElapsedMS > maxElapsedMS {
		return errRecordIdentity
	}
	if record.UsageIdentityDelta < -maxTokenValue || record.UsageIdentityDelta > maxTokenValue {
		return errRecordIdentity
	}
	if record.RejectedIdentifiers < 0 || record.RejectedIdentifiers > maxCountValue {
		return errRecordCount
	}
	return nil
}

// validateRecordVariant admits exactly one variant-less record: the
// stage-level abort record a probe writes when it dies outside any call. It
// must carry a body-free abort reason and no call sequence, because the
// retained record file is the only artifact that survives the canary
// workspace wipe and an interrupted probe still has to explain itself.
func validateRecordVariant(record Record) error {
	switch record.Variant {
	case VariantFull, VariantOptimized:
		return nil
	case "":
		if record.AbortReason != "" && record.Sequence == 0 {
			return nil
		}
	}
	return errRecordIdentity
}

func validateRecordUsage(record Record) error {
	for _, usage := range []*Usage{record.StatsBefore, record.StatsAfter} {
		if usage == nil {
			continue
		}
		if !validUsage(*usage, false) {
			return errRecordUsage
		}
	}
	if len(record.TurnUsages) > maxTurnUsages {
		return errRecordCardinality
	}
	for _, usage := range record.TurnUsages {
		if !validUsage(usage, false) {
			return errRecordUsage
		}
	}
	if record.BranchUsageDelta != nil && !validUsage(*record.BranchUsageDelta, true) {
		return errRecordUsage
	}
	for _, tokens := range []*int64{
		record.TokensBefore, record.TokensAfter,
		record.MaintenanceInputTokens, record.MaintenanceOutputTokens,
	} {
		if tokens != nil && (*tokens < 0 || *tokens > maxTokenValue) {
			return errRecordUsage
		}
	}
	return nil
}

// validUsage bounds one usage tuple. Only a declared delta may be negative.
func validUsage(usage Usage, signed bool) bool {
	lower := int64(0)
	if signed {
		lower = -maxTokenValue
	}
	for _, value := range [...]int64{usage.Input, usage.Output, usage.CacheRead, usage.CacheWrite, usage.Total} {
		if value < lower || value > maxTokenValue {
			return false
		}
	}
	return true
}

func validateRecordEnums(record Record) error {
	if record.AbortReason != "" && !validReasonToken(record.AbortReason) {
		return errRecordEnum
	}
	if record.Kind == KindCall {
		if record.Outcome != "" || record.Method != "" || record.Refusal != "" || record.AttemptCoverage != "" {
			return errRecordScope
		}
		return nil
	}
	switch record.Outcome {
	case OutcomeCompleted, OutcomeRefused, OutcomeFailed:
	default:
		return errRecordEnum
	}
	switch record.Method {
	case MethodNone, MethodRemote, MethodSnapcompact:
	default:
		return errRecordEnum
	}
	switch record.Refusal {
	case RefusalNone, RefusalTooSmall, RefusalNoMessages, RefusalWouldNotReduce:
	default:
		return errRecordEnum
	}
	switch record.AttemptCoverage {
	case AttemptCoverageUnknown, AttemptCoverageComplete, AttemptCoverageLocalOnly:
	default:
		return errRecordEnum
	}
	return nil
}

func validateRecordMetadata(record Record) error {
	if record.CompactionImages < 0 || record.CompactionImages > maxCountValue {
		return errRecordCount
	}
	for _, names := range [][]string{record.MaintenanceUsageKeys, record.UIRequestMethods} {
		if len(names) > maxIdentifierList {
			return errRecordCardinality
		}
		for _, name := range names {
			if !ValidIdentifier(name) {
				return errRecordIdentifier
			}
		}
	}
	for _, histogram := range []map[string]int{record.Roles, record.ContentTypes} {
		if len(histogram) > maxHistogramKeys {
			return errRecordCardinality
		}
		for name, count := range histogram {
			if !ValidIdentifier(name) {
				return errRecordIdentifier
			}
			if count < 0 || count > maxCountValue {
				return errRecordCount
			}
		}
	}
	return nil
}

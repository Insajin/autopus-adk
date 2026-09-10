// Package ompprobe carries the bounded, body-free metadata produced by the
// SPEC-OMP-007 compaction probe (REQ-PROBE-001) together with the
// descriptor-safe exporter that republishes it outside the disposable canary
// workspace.
//
// The schema is deliberately closed. Every field is a number, a bool, a closed
// enum token, or a metadata identifier matching IdentifierPattern, so a probe
// record cannot represent prompt, assistant, tool or credential bytes even
// when the producing runtime is wrong. Diagnostic strings are separate from
// this schema on purpose: nothing here is redacted, because nothing here can
// hold a body in the first place.
package ompprobe

// RecordFileName is the only file name a Recorder writes inside its directory.
const RecordFileName = "probe.jsonl"

// Record kinds. A call record carries the per-call measurement, a compaction
// record carries what one compaction attempt did.
const (
	KindCall       = "call"
	KindCompaction = "compaction"
)

// Cohort variants: A is the full-history session, B the optimized session.
const (
	VariantFull      = "A"
	VariantOptimized = "B"
)

// Compaction methods, matching the verified effective chain vocabulary.
const (
	MethodNone        = "none"
	MethodRemote      = "remote"
	MethodSnapcompact = "snapcompact"
)

// Compaction refusal classes.
const (
	RefusalNone           = "none"
	RefusalTooSmall       = "too_small"
	RefusalNoMessages     = "no_messages"
	RefusalWouldNotReduce = "would_not_reduce"
)

// Compaction outcomes.
const (
	OutcomeCompleted = "completed"
	OutcomeRefused   = "refused"
	OutcomeFailed    = "failed"
)

// Provider attempt coverage. A final success alone never establishes
// AttemptCoverageComplete; see REQ-MEASURE-001.
const (
	AttemptCoverageUnknown   = "unknown"
	AttemptCoverageComplete  = "complete"
	AttemptCoverageLocalOnly = "local_only"
)

// Conservative record bounds shared by the writer and the exporter. A cohort
// is 40 calls with at most one compaction each, so 128 records leaves headroom
// without letting a runaway producer fill the retained path.
const (
	MaxRecords     = 128
	MaxRecordBytes = 64 * 1024
	MaxTotalBytes  = 1 << 20
)

// Usage is one provider usage tuple in tokens.
type Usage struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
	Total      int64 `json:"total"`
}

// Record is one probe observation. Identity and count fields are always
// serialized; everything else is omitted when unset.
//
// Only BranchUsageDelta and UsageIdentityDelta may be negative: they are
// declared signed deltas. Every other number is a non-negative measurement.
type Record struct {
	Kind               string  `json:"kind"`
	Sequence           int     `json:"sequence"`
	Variant            string  `json:"variant"`
	SessionSequence    int     `json:"session_sequence"`
	SessionSegment     int     `json:"session_segment"`
	StatsBefore        *Usage  `json:"stats_before,omitempty"`
	StatsAfter         *Usage  `json:"stats_after,omitempty"`
	TurnUsages         []Usage `json:"turn_usages,omitempty"`
	BranchUsageDelta   *Usage  `json:"branch_usage_delta,omitempty"`
	UsageIdentityDelta int64   `json:"usage_identity_delta"`
	ElapsedMS          int64   `json:"elapsed_ms"`

	Outcome         string `json:"outcome,omitempty"`
	Method          string `json:"method,omitempty"`
	Refusal         string `json:"refusal,omitempty"`
	AbortReason     string `json:"abort_reason,omitempty"`
	AttemptCoverage string `json:"attempt_coverage,omitempty"`

	TokensBefore            *int64   `json:"tokens_before,omitempty"`
	TokensAfter             *int64   `json:"tokens_after,omitempty"`
	MaintenanceUsagePresent bool     `json:"maintenance_usage_present,omitempty"`
	MaintenanceUsageKeys    []string `json:"maintenance_usage_keys,omitempty"`
	MaintenanceInputTokens  *int64   `json:"maintenance_input_tokens,omitempty"`
	MaintenanceOutputTokens *int64   `json:"maintenance_output_tokens,omitempty"`

	CompactionImages int `json:"compaction_images,omitempty"`
	// Counts are authenticated received events, not provider attempts or costs.
	PreCheckpoints      int            `json:"pre_checkpoints,omitempty"`
	PostCheckpoints     int            `json:"post_checkpoints,omitempty"`
	UIRequestMethods    []string       `json:"ui_request_methods,omitempty"`
	Roles               map[string]int `json:"roles,omitempty"`
	ContentTypes        map[string]int `json:"content_types,omitempty"`
	RejectedIdentifiers int            `json:"rejected_identifiers,omitempty"`
}

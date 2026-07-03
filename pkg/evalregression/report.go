// Package evalregression provides a deterministic, read-only CI gate that
// consumes the eval_regression_report.v1 artifact produced by the Primary SPEC
// (SPEC-COMPANY-OPS-EVAL-001, REQ-COE-EXPORT-001). It recomputes nothing: it
// surfaces the already-computed verdict and fails closed on any artifact that is
// malformed, stale, or unsafe.
package evalregression

import "time"

// EvalRegressionReportSchemaV1 is the exact schema_version this gate accepts. It
// mirrors the Primary producer's EvalRegressionReportVersion in
// Autopus/backend/internal/models/eval_regression_report.go. Any other value is
// treated as an invalid artifact (REQ-ECI-INVALID-001).
const EvalRegressionReportSchemaV1 = "eval_regression_report.v1"

// EvalRegressionReportV1 is the consumer view of the Primary producer envelope.
// It is a read-only mirror re-declaring ONLY the flat fields the gate reads; the
// producer single-sources the comparison math (REQ-ECI-SEAM-001, N2). The json
// tags are the EXACT flat tags emitted by the Primary — do not nest.
//
// Strict decoding with json.Decoder.DisallowUnknownFields() happens in the CLI
// loader (REQ-ECI-STRICT-001), not here; this struct only declares the shape.
type EvalRegressionReportV1 struct {
	SchemaVersion string `json:"schema_version"`

	// Verdict fields, copied verbatim from the producer (no recompute here).
	Blocked           bool    `json:"blocked"`
	RegressionDelta   float64 `json:"regression_delta"`
	AttributedVersion string  `json:"attributed_version"`
	ComparisonScope   string  `json:"comparison_scope"`
	ThresholdMetric   string  `json:"threshold_metric"`
	ThresholdValue    float64 `json:"threshold_value"`
	Reason            string  `json:"reason"`

	// BaselineRef is a safe reference to the compared baseline, not the raw baseline.
	BaselineRef string `json:"baseline_ref"`

	// ProducedAt is the producer-injected timestamp used for the freshness axis.
	ProducedAt time.Time `json:"produced_at"`

	// WorkspaceScope is the single workspace the verdict was computed within.
	WorkspaceScope string `json:"workspace_scope"`

	// Redaction fields. RawPayloadPresent MUST be false for a trusted artifact.
	RawPayloadPresent bool   `json:"raw_payload_present"`
	RedactionStatus   string `json:"redaction_status"`
	RetentionClass    string `json:"retention_class"`
}

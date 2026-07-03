package evalregression

import (
	"encoding/json"
	"testing"
	"time"
)

// TestEvalRegressionReportSchemaConstant pins the exact schema identifier the
// consumer accepts, matching the Primary producer's EvalRegressionReportVersion.
func TestEvalRegressionReportSchemaConstant(t *testing.T) {
	if EvalRegressionReportSchemaV1 != "eval_regression_report.v1" {
		t.Fatalf("schema constant = %q, want eval_regression_report.v1", EvalRegressionReportSchemaV1)
	}
}

// TestEvalRegressionReportJSONTags verifies the consumer view round-trips the
// EXACT flat json tags emitted by the Primary producer envelope. This is the
// cross-check against Autopus/backend/internal/models/eval_regression_report.go.
func TestEvalRegressionReportJSONTags(t *testing.T) {
	produced := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	raw := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": true,
		"regression_delta": 0.30,
		"attributed_version": "candidate",
		"comparison_scope": "workspace",
		"threshold_metric": "pass_rate",
		"threshold_value": 0.10,
		"reason": "regression exceeds threshold",
		"baseline_ref": "baseline-abc",
		"produced_at": "2026-07-03T12:00:00Z",
		"workspace_scope": "ws-1",
		"raw_payload_present": false,
		"redaction_status": "redacted",
		"retention_class": "standard"
	}`

	var got EvalRegressionReportV1
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal producer envelope: %v", err)
	}

	if got.SchemaVersion != "eval_regression_report.v1" {
		t.Errorf("SchemaVersion = %q", got.SchemaVersion)
	}
	if !got.Blocked {
		t.Errorf("Blocked = false, want true")
	}
	if got.RegressionDelta != 0.30 {
		t.Errorf("RegressionDelta = %v, want 0.30", got.RegressionDelta)
	}
	if got.AttributedVersion != "candidate" {
		t.Errorf("AttributedVersion = %q, want candidate", got.AttributedVersion)
	}
	if got.ComparisonScope != "workspace" {
		t.Errorf("ComparisonScope = %q", got.ComparisonScope)
	}
	if got.ThresholdMetric != "pass_rate" {
		t.Errorf("ThresholdMetric = %q", got.ThresholdMetric)
	}
	if got.ThresholdValue != 0.10 {
		t.Errorf("ThresholdValue = %v, want 0.10", got.ThresholdValue)
	}
	if got.Reason != "regression exceeds threshold" {
		t.Errorf("Reason = %q", got.Reason)
	}
	if got.BaselineRef != "baseline-abc" {
		t.Errorf("BaselineRef = %q", got.BaselineRef)
	}
	if !got.ProducedAt.Equal(produced) {
		t.Errorf("ProducedAt = %v, want %v", got.ProducedAt, produced)
	}
	if got.WorkspaceScope != "ws-1" {
		t.Errorf("WorkspaceScope = %q", got.WorkspaceScope)
	}
	if got.RawPayloadPresent {
		t.Errorf("RawPayloadPresent = true, want false")
	}
	if got.RedactionStatus != "redacted" {
		t.Errorf("RedactionStatus = %q", got.RedactionStatus)
	}
	if got.RetentionClass != "standard" {
		t.Errorf("RetentionClass = %q", got.RetentionClass)
	}
}

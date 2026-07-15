package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTelemetryEfficiency_S19S20S21EvidenceProducesEligibleCanary(t *testing.T) {
	evidence := efficiencyEvidenceFixture(t)
	path := writeEfficiencyEvidence(t, evidence)

	result, raw, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
		"--evidence-json", path, "--format", "json")
	if err != nil {
		t.Fatalf("efficiency command failed: %v", err)
	}
	if result.Version != telemetryEfficiencyResultVersion {
		t.Fatalf("result version = %d", result.Version)
	}
	if result.Measurement.ActualUsageCapturePct != 95 || result.Measurement.MeasurementGate != "PASS" {
		t.Fatalf("measurement = %+v", result.Measurement)
	}
	if result.Comparison.PairedTaskCount != 2 || result.Comparison.MedianPairedRawReductionPct != 30 {
		t.Fatalf("comparison = %+v", result.Comparison)
	}
	if result.Comparison.ExpectedTaskCount != 2 || result.Comparison.PairedExpectedTaskCount != 2 || !result.Comparison.ExpectedCorpusComplete {
		t.Fatalf("expected corpus completeness = %+v", result.Comparison)
	}
	if result.Promotion.RolloutDecision != "ELIGIBLE_NEXT_CANARY" {
		t.Fatalf("promotion = %+v", result.Promotion)
	}
	if result.RolloutReceipt.Decision != "CANARY" || result.RolloutReceipt.ActiveProfile != "compact_ultra" || result.RolloutReceipt.FullDepth {
		t.Fatalf("rollout receipt = %+v", result.RolloutReceipt)
	}
	for _, forbidden := range []string{evidence.Rollout.ExperimentID, t.TempDir(), "prompt", "response"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("JSON output leaked %q: %s", forbidden, raw)
		}
	}
}

func TestTelemetryEfficiency_DefaultsToShadowAndBlocksIneligibleCanary(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*telemetryEfficiencyEvidence)
	}{
		{name: "receipt kind omitted", mutate: func(e *telemetryEfficiencyEvidence) { e.Rollout.ReceiptKind = "" }},
		{name: "requested canary is ineligible", mutate: func(e *telemetryEfficiencyEvidence) { e.PolicyParityPassed = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := efficiencyEvidenceFixture(t)
			test.mutate(&evidence)
			result, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
				"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json")
			if err != nil {
				t.Fatal(err)
			}
			if result.RolloutReceipt.Decision != "SHADOW" || result.RolloutReceipt.ActiveProfile != "full_ultra" || !result.RolloutReceipt.FullDepth {
				t.Fatalf("ineligible policy escaped full shadow: %+v", result.RolloutReceipt)
			}
		})
	}
}

func TestTelemetryEfficiency_HighRiskEligibleCanaryStaysFullWithoutAuditIdentity(t *testing.T) {
	evidence := efficiencyEvidenceFixture(t)
	evidence.Rollout.RiskTier = "critical"
	evidence.Rollout.AuditTaskHash = ""
	evidence.Rollout.AuditRatePercent = 0

	result, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
		"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if result.RolloutReceipt.Decision != "CANARY" || result.RolloutReceipt.ReceiptKind != "canary" ||
		result.RolloutReceipt.ActiveProfile != "full_ultra" || !result.RolloutReceipt.FullDepth ||
		result.RolloutReceipt.SelectionReason != "risk_requires_full_depth" {
		t.Fatalf("high-risk canary downshifted or shadowed: %+v", result.RolloutReceipt)
	}
}

func TestTelemetryEfficiency_StrictBoundedEvidence(t *testing.T) {
	valid, err := json.Marshal(efficiencyEvidenceFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(valid, &object); err != nil {
		t.Fatal(err)
	}
	object["prompt"] = "/private/customer/raw-prompt-secret"
	unknown, _ := json.Marshal(object)

	tests := []struct {
		name   string
		data   []byte
		format string
	}{
		{name: "malformed", data: []byte(`{"version":`)},
		{name: "unknown field", data: unknown},
		{name: "trailing value", data: append(append([]byte{}, valid...), []byte(` {}`)...)},
		{name: "oversized", data: bytes.Repeat([]byte(" "), maxTelemetryEfficiencyEvidenceBytes+1)},
		{name: "unsupported format", data: valid, format: "yaml"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "raw-prompt-secret.json")
			if err := os.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			format := test.format
			if format == "" {
				format = "json"
			}
			_, raw, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
				"--evidence-json", path, "--format", format)
			if err == nil {
				t.Fatal("expected strict evidence failure")
			}
			combined := raw + err.Error()
			for _, secret := range []string{"raw-prompt-secret", "/private/customer", path} {
				if strings.Contains(combined, secret) {
					t.Fatalf("failure leaked %q: %s", secret, combined)
				}
			}
		})
	}
}

func TestTelemetryEfficiency_RejectsUserPrecomputedDecision(t *testing.T) {
	valid, err := json.Marshal(efficiencyEvidenceFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(valid, &object); err != nil {
		t.Fatal(err)
	}
	object["promotion"] = map[string]any{"rollout_decision": "ELIGIBLE_NEXT_CANARY"}
	data, _ := json.Marshal(object)
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(), "--evidence-json", path, "--format", "json"); err == nil {
		t.Fatal("precomputed decision must not be accepted")
	}
}

func TestTelemetryEfficiency_HumanOutputDerivesFromResult(t *testing.T) {
	evidence := efficiencyEvidenceFixture(t)
	_, raw, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(),
		"--evidence-json", writeEfficiencyEvidence(t, evidence), "--format", "human")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"measurement=PASS", "paired_tasks=2", "median_reduction=30.000%", "promotion=ELIGIBLE_NEXT_CANARY", "receipt=CANARY"} {
		if !strings.Contains(raw, field) {
			t.Fatalf("human output %q missing %q", raw, field)
		}
	}
}

func TestTelemetryEfficiency_RejectsArgumentsBeforeReadingEvidence(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "positional argument", args: []string{"unexpected"}},
		{name: "missing evidence path", args: []string{"--format", "json"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := executeEfficiencyCommand(t, newTelemetryEfficiencyCmd(), test.args...); err == nil {
				t.Fatal("invalid command arguments must fail")
			}
		})
	}
}

func TestTelemetryEfficiency_IsRegisteredWithoutChangingCompare(t *testing.T) {
	cmd := newTelemetryCmd()
	if found, _, err := cmd.Find([]string{"efficiency"}); err != nil || found == cmd {
		t.Fatalf("efficiency command not registered: found=%v err=%v", found, err)
	}
	if found, _, err := cmd.Find([]string{"compare"}); err != nil || found == cmd {
		t.Fatalf("compare command changed or missing: found=%v err=%v", found, err)
	}
}

func executeEfficiencyCommand(t *testing.T, cmd *cobra.Command, args ...string) (telemetryEfficiencyResult, string, error) {
	t.Helper()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)
	err := cmd.Execute()
	var result telemetryEfficiencyResult
	if err == nil && strings.HasPrefix(strings.TrimSpace(output.String()), "{") {
		if decodeErr := json.Unmarshal(output.Bytes(), &result); decodeErr != nil {
			return result, output.String(), decodeErr
		}
	}
	return result, output.String(), err
}

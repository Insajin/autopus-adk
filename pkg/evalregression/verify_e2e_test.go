package evalregression

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const e2eKeyID = "egl-e2e-1"

var e2eFixedNow = time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

func e2eReportJSON(blocked bool, producedAt, attributedVersion string) []byte {
	reason := "candidate within threshold"
	delta := "0.04"
	if blocked {
		reason = "regression exceeds threshold"
		delta = "0.30"
	}
	return []byte(`{"schema_version":"eval_regression_report.v1","blocked":` +
		boolStr(blocked) + `,"regression_delta":` + delta +
		`,"attributed_version":"` + attributedVersion +
		`","comparison_scope":"workspace","threshold_metric":"pass_rate","threshold_value":0.10,` +
		`"reason":"` + reason +
		`","baseline_ref":"baseline-e2e","produced_at":"` + producedAt +
		`","workspace_scope":"ws-e2e","raw_payload_present":false,` +
		`"redaction_status":"redacted","retention_class":"standard"}`)
}

func evaluateE2E(t *testing.T, reportBytes, attBytes []byte, trusted map[string]ed25519.PublicKey) (string, int) {
	t.Helper()
	if reason, ok := VerifyEvalRegressionArtifact(reportBytes, attBytes, trusted); !ok {
		return reason, 1
	}
	var report EvalRegressionReportV1
	if err := jsonUnmarshalReport(reportBytes, &report); err != nil {
		t.Fatalf("verified e2e report should decode: %v", err)
	}
	decision := EvaluateEvalRegressionGate(report, e2eFixedNow, 24*time.Hour)
	return decision.Reason, decision.ExitCode
}

func jsonUnmarshalReport(data []byte, report *EvalRegressionReportV1) error {
	return json.Unmarshal(data, report)
}

func TestEvalRegressionLiveGateE2EReasons(t *testing.T) {
	producedAt := e2eFixedNow.Add(-1 * time.Hour).Format(time.RFC3339)
	headSHA := "0123456789abcdef0123456789abcdef01234567"

	t.Run("green pass", func(t *testing.T) {
		priv, trusted := newSigner(t)
		report := e2eReportJSON(false, producedAt, "candidate")
		att := signBytes(t, report, verifyKeyID, priv)

		reason, exitCode := evaluateE2E(t, report, att, trusted)
		if reason != reasonOK || exitCode != 0 {
			t.Fatalf("green pass: expected (%q, 0), got (%q, %d)", reasonOK, reason, exitCode)
		}
	})

	t.Run("tampered byte", func(t *testing.T) {
		priv, trusted := newSigner(t)
		original := e2eReportJSON(true, producedAt, headSHA)
		att := signBytes(t, original, verifyKeyID, priv)
		mutated := []byte(strings.Replace(string(original), `"blocked":true`, `"blocked":false`, 1))
		if string(mutated) == string(original) {
			t.Fatalf("tampered byte: setup failed to mutate blocked")
		}

		reason, exitCode := evaluateE2E(t, mutated, att, trusted)
		if reason != reasonSignatureInvalid || exitCode != 1 {
			t.Fatalf("tampered byte: expected (%q, 1), got (%q, %d)", reasonSignatureInvalid, reason, exitCode)
		}
	})

	t.Run("missing attestation", func(t *testing.T) {
		_, trusted := newSigner(t)
		report := e2eReportJSON(false, producedAt, "candidate")

		reason, exitCode := evaluateE2E(t, report, nil, trusted)
		if reason != reasonArtifactUnsigned || exitCode != 1 {
			t.Fatalf("missing attestation: expected (%q, 1), got (%q, %d)", reasonArtifactUnsigned, reason, exitCode)
		}
	})

	t.Run("unknown key", func(t *testing.T) {
		priv, _ := newSigner(t)
		report := e2eReportJSON(false, producedAt, "candidate")
		att := signBytes(t, report, e2eKeyID, priv)

		reason, exitCode := evaluateE2E(t, report, att, map[string]ed25519.PublicKey{})
		if reason != reasonSignatureKeyUnknown || exitCode != 1 {
			t.Fatalf("unknown key: expected (%q, 1), got (%q, %d)", reasonSignatureKeyUnknown, reason, exitCode)
		}
	})

	t.Run("blocked head sha", func(t *testing.T) {
		priv, trusted := newSigner(t)
		report := e2eReportJSON(true, producedAt, headSHA)
		att := signBytes(t, report, verifyKeyID, priv)

		reason, exitCode := evaluateE2E(t, report, att, trusted)
		if reason != reasonBlocked || exitCode != 1 {
			t.Fatalf("blocked head sha: expected (%q, 1), got (%q, %d)", reasonBlocked, reason, exitCode)
		}
	})
}

func TestCommittedAllowlistEmptyAndDefensiveForE2E(t *testing.T) {
	keys := CommittedEvalRegressionPublicKeys()
	if len(keys) != 0 {
		t.Fatalf("committed allowlist length = %d, want 0", len(keys))
	}

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate public key: %v", err)
	}
	keys["egl-should-not-stick"] = pub

	again := CommittedEvalRegressionPublicKeys()
	if len(again) != 0 {
		t.Fatalf("committed allowlist mutated through defensive copy: length = %d, want 0", len(again))
	}

	priv, _ := newSigner(t)
	report := e2eReportJSON(false, e2eFixedNow.Add(-1*time.Hour).Format(time.RFC3339), "candidate")
	att := signBytes(t, report, e2eKeyID, priv)
	reason, exitCode := evaluateE2E(t, report, att, again)
	if reason != reasonSignatureKeyUnknown || exitCode != 1 {
		t.Fatalf("empty committed allowlist: expected (%q, 1), got (%q, %d)", reasonSignatureKeyUnknown, reason, exitCode)
	}
}

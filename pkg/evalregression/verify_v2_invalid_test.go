package evalregression

import (
	"crypto/ed25519"
	"testing"
)

func TestEvalRegressionStrictVerifyReasonsStable(t *testing.T) {
	reasons := EvalRegressionStrictVerifyReasons()
	if reasons["policy_invalid"] != "attestation_policy_invalid" {
		t.Fatalf("policy_invalid reason = %q", reasons["policy_invalid"])
	}
	if reasons["policy_mismatch"] != "attestation_policy_mismatch" {
		t.Fatalf("policy_mismatch reason = %q", reasons["policy_mismatch"])
	}
	if len(reasons) != 2 {
		t.Fatalf("strict reason count = %d, want 2", len(reasons))
	}
}

func TestVerifyEvalRegressionArtifactV2StrictInvalidEnvelopeBoundaries(t *testing.T) {
	policy := v2TestPolicy()
	priv, pub := newV2Signer(t)
	report := v2ReportJSON(policy.WorkspaceScope, v2TestProducedAt)
	base := signV2Attestation(t, report, policy, priv)
	trusted := map[string]ed25519.PublicKey{policy.ExpectedKeyID: pub}
	cases := map[string]func(*EvalRegressionAttestationV2){
		"wrong schema": func(att *EvalRegressionAttestationV2) {
			att.SchemaVersion = "eval_regression_attestation.v99"
		},
		"wrong algorithm": func(att *EvalRegressionAttestationV2) {
			att.Algorithm = "rsa"
		},
		"bad signature base64": func(att *EvalRegressionAttestationV2) {
			att.SignatureB64 = "not-base64"
		},
		"empty produced at": func(att *EvalRegressionAttestationV2) {
			att.ProducedAt = ""
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			att := base
			mutate(&att)
			reason, ok := VerifyEvalRegressionArtifactV2Strict(
				report,
				marshalV2Attestation(t, att),
				trusted,
				policy,
			)
			if ok || reason != reasonSignatureInvalid {
				t.Fatalf("expected (%q, false), got (%q, %v)", reasonSignatureInvalid, reason, ok)
			}
		})
	}
}

func TestVerifyEvalRegressionArtifactV2StrictInvalidPolicyPrecedesArtifact(t *testing.T) {
	reason, ok := VerifyEvalRegressionArtifactV2Strict(nil, nil, nil, EvalRegressionAttestationPolicyV2{})
	if ok || reason != reasonAttestationPolicyInvalid {
		t.Fatalf("expected (%q, false), got (%q, %v)", reasonAttestationPolicyInvalid, reason, ok)
	}
}

func TestVerifyEvalRegressionArtifactV2StrictUnreadableSignedReportContextFailsClosed(t *testing.T) {
	policy := v2TestPolicy()
	priv, pub := newV2Signer(t)
	report := []byte(`{"produced_at":"2026-07-17T03:00:00Z"} trailing`)
	att := signV2Attestation(t, report, policy, priv)
	reason, ok := VerifyEvalRegressionArtifactV2Strict(
		report,
		marshalV2Attestation(t, att),
		map[string]ed25519.PublicKey{policy.ExpectedKeyID: pub},
		policy,
	)
	if ok || reason != reasonAttestationPolicyMismatch {
		t.Fatalf("expected (%q, false), got (%q, %v)", reasonAttestationPolicyMismatch, reason, ok)
	}
}

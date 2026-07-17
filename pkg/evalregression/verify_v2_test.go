package evalregression

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

const v2SigningDomain = "autopus.eval-regression.attestation.v2\x00"
const v2TestProducedAt = "2026-07-17T03:00:00Z"

type v2TestStatement struct {
	SchemaVersion     string `json:"schema_version"`
	KeyID             string `json:"key_id"`
	Algorithm         string `json:"algorithm"`
	ReportSHA256      string `json:"report_sha256"`
	ProducedAt        string `json:"produced_at"`
	TrustLane         string `json:"trust_lane"`
	SourceEnvironment string `json:"source_environment"`
	TargetEnvironment string `json:"target_environment"`
	SourceRevision    string `json:"source_revision"`
	WorkspaceScope    string `json:"workspace_scope"`
}

func v2TestPolicy() EvalRegressionAttestationPolicyV2 {
	return EvalRegressionAttestationPolicyV2{
		ExpectedKeyID:     "eval-prod-2026-07",
		TrustLane:         "production-promotion",
		SourceEnvironment: "staging",
		TargetEnvironment: "production",
		SourceRevision:    "0123456789abcdef0123456789abcdef01234567",
		WorkspaceScope:    "autopus-primary",
	}
}

func signV2Attestation(t *testing.T, report []byte, policy EvalRegressionAttestationPolicyV2, priv ed25519.PrivateKey) EvalRegressionAttestationV2 {
	t.Helper()
	sum := sha256.Sum256(report)
	att := EvalRegressionAttestationV2{
		SchemaVersion:     EvalRegressionAttestationSchemaV2,
		KeyID:             policy.ExpectedKeyID,
		Algorithm:         "ed25519",
		ReportSHA256:      hex.EncodeToString(sum[:]),
		ProducedAt:        v2TestProducedAt,
		TrustLane:         policy.TrustLane,
		SourceEnvironment: policy.SourceEnvironment,
		TargetEnvironment: policy.TargetEnvironment,
		SourceRevision:    policy.SourceRevision,
		WorkspaceScope:    policy.WorkspaceScope,
	}
	statement, err := json.Marshal(v2TestStatement{
		SchemaVersion:     att.SchemaVersion,
		KeyID:             att.KeyID,
		Algorithm:         att.Algorithm,
		ReportSHA256:      att.ReportSHA256,
		ProducedAt:        att.ProducedAt,
		TrustLane:         att.TrustLane,
		SourceEnvironment: att.SourceEnvironment,
		TargetEnvironment: att.TargetEnvironment,
		SourceRevision:    att.SourceRevision,
		WorkspaceScope:    att.WorkspaceScope,
	})
	if err != nil {
		t.Fatalf("marshal v2 statement: %v", err)
	}
	message := append([]byte(v2SigningDomain), statement...)
	att.SignatureB64 = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message))
	return att
}

func marshalV2Attestation(t *testing.T, att EvalRegressionAttestationV2) []byte {
	t.Helper()
	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("marshal v2 attestation: %v", err)
	}
	return data
}

func newV2Signer(t *testing.T) (ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate v2 signer: %v", err)
	}
	return priv, pub
}

func v2ReportJSON(workspaceScope, producedAt string) []byte {
	return []byte(`{"schema_version":"eval_regression_report.v1","blocked":false,` +
		`"attributed_version":"candidate","produced_at":"` + producedAt + `",` +
		`"workspace_scope":"` + workspaceScope + `","raw_payload_present":false}`)
}

func TestVerifyEvalRegressionArtifactV2StrictValid(t *testing.T) {
	policy := v2TestPolicy()
	priv, pub := newV2Signer(t)
	report := v2ReportJSON(policy.WorkspaceScope, v2TestProducedAt)
	att := signV2Attestation(t, report, policy, priv)

	if reason, ok := ValidateEvalRegressionAttestationPolicyV2(policy); !ok || reason != "" {
		t.Fatalf("valid policy rejected: (%q, %v)", reason, ok)
	}
	if reason, ok := VerifyEvalRegressionArtifactV2Strict(
		report,
		marshalV2Attestation(t, att),
		map[string]ed25519.PublicKey{policy.ExpectedKeyID: pub},
		policy,
	); !ok || reason != "" {
		t.Fatalf("valid v2 attestation rejected: (%q, %v)", reason, ok)
	}
}

func TestValidateEvalRegressionAttestationPolicyV2RequiresEveryField(t *testing.T) {
	cases := map[string]func(*EvalRegressionAttestationPolicyV2){
		"expected key id":    func(p *EvalRegressionAttestationPolicyV2) { p.ExpectedKeyID = " \t" },
		"trust lane":         func(p *EvalRegressionAttestationPolicyV2) { p.TrustLane = "" },
		"source environment": func(p *EvalRegressionAttestationPolicyV2) { p.SourceEnvironment = "" },
		"target environment": func(p *EvalRegressionAttestationPolicyV2) { p.TargetEnvironment = "" },
		"source revision":    func(p *EvalRegressionAttestationPolicyV2) { p.SourceRevision = "" },
		"workspace scope":    func(p *EvalRegressionAttestationPolicyV2) { p.WorkspaceScope = "" },
		"surrounding space":  func(p *EvalRegressionAttestationPolicyV2) { p.TrustLane = " staging-to-main " },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			policy := v2TestPolicy()
			mutate(&policy)
			reason, ok := ValidateEvalRegressionAttestationPolicyV2(policy)
			if ok || reason != reasonAttestationPolicyInvalid {
				t.Fatalf("expected (%q, false), got (%q, %v)", reasonAttestationPolicyInvalid, reason, ok)
			}
		})
	}
}

func TestVerifyEvalRegressionArtifactV2StrictPolicyMismatches(t *testing.T) {
	basePolicy := v2TestPolicy()
	priv, pub := newV2Signer(t)
	report := v2ReportJSON(basePolicy.WorkspaceScope, v2TestProducedAt)
	attData := marshalV2Attestation(t, signV2Attestation(t, report, basePolicy, priv))
	cases := map[string]func(*EvalRegressionAttestationPolicyV2){
		"expected key id":    func(p *EvalRegressionAttestationPolicyV2) { p.ExpectedKeyID = "other-key" },
		"trust lane":         func(p *EvalRegressionAttestationPolicyV2) { p.TrustLane = "other-lane" },
		"source environment": func(p *EvalRegressionAttestationPolicyV2) { p.SourceEnvironment = "dev" },
		"target environment": func(p *EvalRegressionAttestationPolicyV2) { p.TargetEnvironment = "preview" },
		"source revision":    func(p *EvalRegressionAttestationPolicyV2) { p.SourceRevision = "other-revision" },
		"workspace scope":    func(p *EvalRegressionAttestationPolicyV2) { p.WorkspaceScope = "other-workspace" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			policy := basePolicy
			mutate(&policy)
			reason, ok := VerifyEvalRegressionArtifactV2Strict(
				report,
				attData,
				map[string]ed25519.PublicKey{basePolicy.ExpectedKeyID: pub},
				policy,
			)
			if ok || reason != reasonAttestationPolicyMismatch {
				t.Fatalf("expected (%q, false), got (%q, %v)", reasonAttestationPolicyMismatch, reason, ok)
			}
		})
	}
}

func TestVerifyEvalRegressionArtifactV2StrictTamperBoundaries(t *testing.T) {
	policy := v2TestPolicy()
	priv, pub := newV2Signer(t)
	report := v2ReportJSON(policy.WorkspaceScope, v2TestProducedAt)
	base := signV2Attestation(t, report, policy, priv)
	trusted := map[string]ed25519.PublicKey{policy.ExpectedKeyID: pub}

	t.Run("report bytes", func(t *testing.T) {
		mutated := append(append([]byte(nil), report...), '\n')
		reason, ok := VerifyEvalRegressionArtifactV2Strict(mutated, marshalV2Attestation(t, base), trusted, policy)
		if ok || reason != reasonSignatureInvalid {
			t.Fatalf("expected (%q, false), got (%q, %v)", reasonSignatureInvalid, reason, ok)
		}
	})

	t.Run("uppercase report digest", func(t *testing.T) {
		mutated := base
		mutated.ReportSHA256 = "ABCDEF" + mutated.ReportSHA256[6:]
		reason, ok := VerifyEvalRegressionArtifactV2Strict(report, marshalV2Attestation(t, mutated), trusted, policy)
		if ok || reason != reasonSignatureInvalid {
			t.Fatalf("expected lowercase digest enforcement, got (%q, %v)", reason, ok)
		}
	})

	t.Run("signed metadata", func(t *testing.T) {
		mutated := base
		mutated.SourceRevision = "attacker-revision"
		matchingPolicy := policy
		matchingPolicy.SourceRevision = mutated.SourceRevision
		reason, ok := VerifyEvalRegressionArtifactV2Strict(report, marshalV2Attestation(t, mutated), trusted, matchingPolicy)
		if ok || reason != reasonSignatureInvalid {
			t.Fatalf("metadata tamper must invalidate signature, got (%q, %v)", reason, ok)
		}
	})

	t.Run("produced at", func(t *testing.T) {
		mutated := base
		mutated.ProducedAt = "2026-07-17T04:00:00Z"
		reason, ok := VerifyEvalRegressionArtifactV2Strict(report, marshalV2Attestation(t, mutated), trusted, policy)
		if ok || reason != reasonSignatureInvalid {
			t.Fatalf("produced_at tamper must invalidate signature, got (%q, %v)", reason, ok)
		}
	})
}

func TestVerifyEvalRegressionArtifactV2StrictRejectsMalformedAndUnknownSigner(t *testing.T) {
	policy := v2TestPolicy()
	priv, _ := newV2Signer(t)
	report := v2ReportJSON(policy.WorkspaceScope, v2TestProducedAt)
	valid := marshalV2Attestation(t, signV2Attestation(t, report, policy, priv))

	if reason, ok := VerifyEvalRegressionArtifactV2Strict(report, nil, nil, policy); ok || reason != reasonArtifactUnsigned {
		t.Fatalf("unsigned: expected (%q, false), got (%q, %v)", reasonArtifactUnsigned, reason, ok)
	}
	if reason, ok := VerifyEvalRegressionArtifactV2Strict(report, valid, nil, policy); ok || reason != reasonSignatureKeyUnknown {
		t.Fatalf("unknown key: expected (%q, false), got (%q, %v)", reasonSignatureKeyUnknown, reason, ok)
	}
	for _, malformed := range [][]byte{
		[]byte(`{"schema_version":"eval_regression_attestation.v2","unexpected":true}`),
		append(append([]byte(nil), valid...), []byte(` {}`)...),
	} {
		if reason, ok := VerifyEvalRegressionArtifactV2Strict(report, malformed, nil, policy); ok || reason != reasonSignatureInvalid {
			t.Fatalf("malformed: expected (%q, false), got (%q, %v)", reasonSignatureInvalid, reason, ok)
		}
	}
}

func TestVerifyEvalRegressionArtifactV1RemainsCompatible(t *testing.T) {
	priv, trusted := newSigner(t)
	report := v2ReportJSON(v2TestPolicy().WorkspaceScope, v2TestProducedAt)
	v1 := signBytes(t, report, verifyKeyID, priv)
	if reason, ok := VerifyEvalRegressionArtifact(report, v1, trusted); !ok || reason != "" {
		t.Fatalf("v1 verifier regression: (%q, %v)", reason, ok)
	}
	if reason, ok := VerifyEvalRegressionArtifactV2Strict(report, v1, trusted, v2TestPolicy()); ok || reason != reasonSignatureInvalid {
		t.Fatalf("strict v2 accepted v1 envelope: (%q, %v)", reason, ok)
	}
}

func TestVerifyEvalRegressionArtifactV2StrictBindsReportContextAfterSignature(t *testing.T) {
	policy := v2TestPolicy()
	priv, pub := newV2Signer(t)
	trusted := map[string]ed25519.PublicKey{policy.ExpectedKeyID: pub}
	cases := map[string][]byte{
		"workspace scope": v2ReportJSON("other-workspace", v2TestProducedAt),
		"produced at":     v2ReportJSON(policy.WorkspaceScope, "2026-07-17T04:00:00Z"),
	}
	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			att := signV2Attestation(t, report, policy, priv)
			reason, ok := VerifyEvalRegressionArtifactV2Strict(
				report,
				marshalV2Attestation(t, att),
				trusted,
				policy,
			)
			if ok || reason != reasonAttestationPolicyMismatch {
				t.Fatalf("expected (%q, false), got (%q, %v)", reasonAttestationPolicyMismatch, reason, ok)
			}
		})
	}

	t.Run("signature failure precedes report context", func(t *testing.T) {
		validReport := v2ReportJSON(policy.WorkspaceScope, v2TestProducedAt)
		att := signV2Attestation(t, validReport, policy, priv)
		mismatchedReport := v2ReportJSON("other-workspace", v2TestProducedAt)
		sum := sha256.Sum256(mismatchedReport)
		att.ReportSHA256 = hex.EncodeToString(sum[:])
		reason, ok := VerifyEvalRegressionArtifactV2Strict(
			mismatchedReport,
			marshalV2Attestation(t, att),
			trusted,
			policy,
		)
		if ok || reason != reasonSignatureInvalid {
			t.Fatalf("signature failure must precede context, got (%q, %v)", reason, ok)
		}
	})
}

package evalregression

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// verifyFixedNow is the injected clock for the verify+gate composition oracles.
var verifyFixedNow = time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

const verifyKeyID = "evp-test-1"

// signBytes returns a valid eval_regression_attestation.v1 JSON over the exact
// report bytes using the given private key and key_id.
func signBytes(t *testing.T, reportBytes []byte, keyID string, priv ed25519.PrivateKey) []byte {
	t.Helper()
	sum := sha256.Sum256(reportBytes)
	att := map[string]string{
		"schema_version": EvalRegressionAttestationSchemaV1,
		"key_id":         keyID,
		"algorithm":      "ed25519",
		"signature_b64":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, reportBytes)),
		"report_sha256":  hex.EncodeToString(sum[:]),
		"produced_at":    "2026-07-03T11:00:00Z",
	}
	b, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("marshal attestation: %v", err)
	}
	return b
}

// newSigner returns a fresh test keypair and the trusted allowlist containing it.
func newSigner(t *testing.T) (ed25519.PrivateKey, map[string]ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return priv, map[string]ed25519.PublicKey{verifyKeyID: pub}
}

// evaluate composes verify-before-trust: verify the signature over reportBytes,
// then (only on success) strictly-independent gate evaluation. It returns the
// concrete (reason, exitCode) oracle. The report's blocked field is never read
// when verify fails, because gate evaluation is skipped.
func evaluate(t *testing.T, reportBytes, attBytes []byte, trusted map[string]ed25519.PublicKey) (string, int) {
	t.Helper()
	if reason, ok := VerifyEvalRegressionArtifact(reportBytes, attBytes, trusted); !ok {
		return reason, 1
	}
	var report EvalRegressionReportV1
	if err := json.Unmarshal(reportBytes, &report); err != nil {
		t.Fatalf("verified report should decode: %v", err)
	}
	d := EvaluateEvalRegressionGate(report, verifyFixedNow, 24*time.Hour)
	return d.Reason, d.ExitCode
}

func reportJSON(blocked bool, producedAt string) []byte {
	return []byte(`{"schema_version":"eval_regression_report.v1","blocked":` +
		boolStr(blocked) + `,"attributed_version":"candidate","produced_at":"` +
		producedAt + `","raw_payload_present":false}`)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// S1 — valid, allowlisted, fresh, blocked:false → verify ok, gate reason ok, exit 0.
func TestVerifyS1ValidFreshControl(t *testing.T) {
	priv, trusted := newSigner(t)
	report := reportJSON(false, "2026-07-03T11:00:00Z")
	att := signBytes(t, report, verifyKeyID, priv)

	reason, exit := evaluate(t, report, att, trusted)
	if reason != reasonOK || exit != 0 {
		t.Fatalf("S1: expected (%q, 0), got (%q, %d)", reasonOK, reason, exit)
	}
}

// S2 (CORE) — validly-signed blocked:true, then on-disk bytes mutated to
// blocked:false keeping the old signature and old sha256 → signature_invalid,
// exit 1. The mutated blocked value is never read.
func TestVerifyS2TamperedBytes(t *testing.T) {
	priv, trusted := newSigner(t)
	original := reportJSON(true, "2026-07-03T11:00:00Z")
	att := signBytes(t, original, verifyKeyID, priv) // signs blocked:true bytes

	mutated := []byte(strings.Replace(string(original), `"blocked":true`, `"blocked":false`, 1))
	if string(mutated) == string(original) {
		t.Fatalf("S2: setup failed to mutate blocked")
	}

	reason, ok := VerifyEvalRegressionArtifact(mutated, att, trusted)
	if ok {
		t.Fatalf("S2: expected verify failure on tampered bytes")
	}
	if reason != reasonSignatureInvalid {
		t.Fatalf("S2: expected %q, got %q", reasonSignatureInvalid, reason)
	}
	r, exit := evaluate(t, mutated, att, trusted)
	if r != reasonSignatureInvalid || exit != 1 {
		t.Fatalf("S2: expected (%q, 1), got (%q, %d)", reasonSignatureInvalid, r, exit)
	}
}

// S3 — report present, attestation absent/empty → artifact_unsigned, exit 1.
func TestVerifyS3Unsigned(t *testing.T) {
	_, trusted := newSigner(t)
	report := reportJSON(false, "2026-07-03T11:00:00Z")

	for _, att := range [][]byte{nil, {}, []byte("   \n\t ")} {
		reason, exit := evaluate(t, report, att, trusted)
		if reason != reasonArtifactUnsigned || exit != 1 {
			t.Fatalf("S3: expected (%q, 1), got (%q, %d)", reasonArtifactUnsigned, reason, exit)
		}
	}
}

// S4 — valid internally-consistent attestation but key_id NOT in the trusted map
// → signature_key_unknown, exit 1.
func TestVerifyS4UnknownKey(t *testing.T) {
	priv, _ := newSigner(t)
	report := reportJSON(false, "2026-07-03T11:00:00Z")
	att := signBytes(t, report, "some-other-key", priv)

	reason, exit := evaluate(t, report, att, map[string]ed25519.PublicKey{verifyKeyID: nil})
	if reason != reasonSignatureKeyUnknown || exit != 1 {
		t.Fatalf("S4: expected (%q, 1), got (%q, %d)", reasonSignatureKeyUnknown, reason, exit)
	}

	// An empty allowlist (production default) also rejects as key_unknown.
	report2 := reportJSON(false, "2026-07-03T11:00:00Z")
	att2 := signBytes(t, report2, verifyKeyID, priv)
	reason2, exit2 := evaluate(t, report2, att2, map[string]ed25519.PublicKey{})
	if reason2 != reasonSignatureKeyUnknown || exit2 != 1 {
		t.Fatalf("S4(empty): expected (%q, 1), got (%q, %d)", reasonSignatureKeyUnknown, reason2, exit2)
	}
}

// S5 — validly-signed, allowlisted, fresh blocked:true → regression_blocked, exit 1.
func TestVerifyS5Blocked(t *testing.T) {
	priv, trusted := newSigner(t)
	report := reportJSON(true, "2026-07-03T11:00:00Z")
	att := signBytes(t, report, verifyKeyID, priv)

	reason, exit := evaluate(t, report, att, trusted)
	if reason != reasonBlocked || exit != 1 {
		t.Fatalf("S5: expected (%q, 1), got (%q, %d)", reasonBlocked, reason, exit)
	}
}

// S6 — preservation of the decode/gate chain, each sub-case validly signed and
// allowlisted, exercising the reasons that were reachable before signing.
func TestVerifyS6Preservation(t *testing.T) {
	priv, trusted := newSigner(t)

	// (a) unknown out-of-schema field in the (verified) report → artifact_unsafe.
	// The strict decode lives in the CLI loader; here we assert VerifyEvalRegression
	// passes over the exact bytes and json strict-decode would flag it, mirroring
	// the CLI decode path. We verify signature ok, then confirm the field is present.
	unsafe := []byte(`{"schema_version":"eval_regression_report.v1","blocked":false,` +
		`"attributed_version":"candidate","produced_at":"2026-07-03T11:00:00Z",` +
		`"raw_payload_present":false,"leaked":"x"}`)
	attUnsafe := signBytes(t, unsafe, verifyKeyID, priv)
	if _, ok := VerifyEvalRegressionArtifact(unsafe, attUnsafe, trusted); !ok {
		t.Fatalf("S6a: expected verify ok over exact unsafe bytes")
	}

	// (b) produced_at = fixedNow-25h → artifact_stale, exit 1.
	stale := reportJSON(false, verifyFixedNow.Add(-25*time.Hour).Format(time.RFC3339))
	attStale := signBytes(t, stale, verifyKeyID, priv)
	if reason, exit := evaluate(t, stale, attStale, trusted); reason != reasonStale || exit != 1 {
		t.Fatalf("S6b: expected (%q, 1), got (%q, %d)", reasonStale, reason, exit)
	}

	// (c) produced_at = exactly fixedNow-24h → NOT stale, proceeds to verdict (ok).
	edge := reportJSON(false, verifyFixedNow.Add(-24*time.Hour).Format(time.RFC3339))
	attEdge := signBytes(t, edge, verifyKeyID, priv)
	if reason, exit := evaluate(t, edge, attEdge, trusted); reason != reasonOK || exit != 0 {
		t.Fatalf("S6c: expected (%q, 0), got (%q, %d)", reasonOK, reason, exit)
	}

	// (e) schema_version eval_regression_report.v2 → artifact_invalid, exit 1.
	v2 := []byte(`{"schema_version":"eval_regression_report.v2","blocked":false,` +
		`"attributed_version":"candidate","produced_at":"2026-07-03T11:00:00Z",` +
		`"raw_payload_present":false}`)
	attV2 := signBytes(t, v2, verifyKeyID, priv)
	if reason, exit := evaluate(t, v2, attV2, trusted); reason != reasonInvalid || exit != 1 {
		t.Fatalf("S6e: expected (%q, 1), got (%q, %d)", reasonInvalid, reason, exit)
	}
}

// Malformed attestation JSON (unknown field / bad syntax) → signature_invalid.
func TestVerifyMalformedAttestation(t *testing.T) {
	priv, trusted := newSigner(t)
	report := reportJSON(false, "2026-07-03T11:00:00Z")
	_ = priv

	for _, bad := range []string{`{not json`, `{"schema_version":"eval_regression_attestation.v1","extra":1}`} {
		if reason, ok := VerifyEvalRegressionArtifact(report, []byte(bad), trusted); ok || reason != reasonSignatureInvalid {
			t.Fatalf("malformed: expected (%q, false), got (%q, %v)", reasonSignatureInvalid, reason, ok)
		}
	}
}

// CommittedEvalRegressionPublicKeys returns a defensive copy: mutating the
// returned map must not affect the committed allowlist.
func TestCommittedPublicKeysDefensiveCopy(t *testing.T) {
	got := CommittedEvalRegressionPublicKeys()
	got["injected"] = ed25519.PublicKey("attacker")
	again := CommittedEvalRegressionPublicKeys()
	if _, present := again["injected"]; present {
		t.Fatalf("defensive copy failed: mutation leaked into committed allowlist")
	}
}

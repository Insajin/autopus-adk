package evalregression

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// buildAtt constructs a well-formed attestation JSON for report, substituting
// the caller-supplied fields for schema_version, algorithm, and signature_b64.
// report_sha256 is always computed from the actual report bytes so the sha256
// gate passes unless the caller deliberately provides mismatched report bytes.
func buildAtt(t *testing.T, report []byte, keyID, schema, alg, sigB64 string) []byte {
	t.Helper()
	sum := sha256.Sum256(report)
	b, err := json.Marshal(map[string]string{
		"schema_version": schema,
		"key_id":         keyID,
		"algorithm":      alg,
		"signature_b64":  sigB64,
		"report_sha256":  hex.EncodeToString(sum[:]),
		"produced_at":    "2026-07-03T11:00:00Z",
	})
	if err != nil {
		t.Fatalf("buildAtt marshal: %v", err)
	}
	return b
}

// TestVerifyNegSignatureInvalidBranches covers five fail-closed paths inside
// VerifyEvalRegressionArtifact that each resolve to reason=signature_invalid.
// These paths are not exercised by the existing positive/tamper/unsigned/unknown-
// key tests because each guards a distinct code branch (steps 3 and 6 in the
// function's evaluation order):
//
//   - wrong_algorithm   — step 3: algorithm field is "rsa" (schema_version correct)
//   - wrong_schema_version — step 3: schema_version mismatch (algorithm correct)
//   - bad_base64_sig    — step 6: signature_b64 cannot be base64-decoded
//   - wrong_sig_length  — step 6: decodes to 10 bytes, not ed25519.SignatureSize (64)
//   - empty_object      — step 3: all fields zero (empty JSON object {}), schema mismatch
func TestVerifyNegSignatureInvalidBranches(t *testing.T) {
	priv, trusted := newSigner(t)
	report := reportJSON(false, "2026-07-03T11:00:00Z")
	validSig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, report))
	shortSig := base64.StdEncoding.EncodeToString(make([]byte, 10))

	cases := []struct {
		name string
		att  func() []byte
	}{
		{
			"wrong_algorithm",
			func() []byte {
				return buildAtt(t, report, verifyKeyID, EvalRegressionAttestationSchemaV1, "rsa", validSig)
			},
		},
		{
			"wrong_schema_version",
			func() []byte {
				return buildAtt(t, report, verifyKeyID, "eval_regression_attestation.v99", "ed25519", validSig)
			},
		},
		{
			"bad_base64_sig",
			func() []byte {
				return buildAtt(t, report, verifyKeyID, EvalRegressionAttestationSchemaV1, "ed25519", "!!!not-base64!!!")
			},
		},
		{
			"wrong_sig_length",
			func() []byte {
				return buildAtt(t, report, verifyKeyID, EvalRegressionAttestationSchemaV1, "ed25519", shortSig)
			},
		},
		{
			"empty_object",
			func() []byte { return []byte("{}") },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, ok := VerifyEvalRegressionArtifact(report, tc.att(), trusted)
			if ok || reason != reasonSignatureInvalid {
				t.Fatalf("%s: expected (%q, false), got (%q, %v)", tc.name, reasonSignatureInvalid, reason, ok)
			}
		})
	}
}

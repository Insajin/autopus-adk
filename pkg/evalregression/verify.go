package evalregression

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// Signature-verify reason codes emitted BEFORE the strict-decode and the pure
// gate run (SPEC-EVAL-REGRESSION-PROV-001). These are stable machine-readable
// literals asserted by the S1–S6 verify oracles. They are the verify-before-
// trust counterparts to the load/gate reasons in gate.go and eval_regression.go.
const (
	// reasonArtifactUnsigned: a present report has no accompanying attestation
	// (REQ-EVP-UNSIGNED-001). Fail closed — an unsigned artifact is untrusted.
	reasonArtifactUnsigned = "artifact_unsigned"
	// reasonSignatureInvalid: the attestation is malformed, the sha256 does not
	// match the on-disk report bytes, or ed25519.Verify fails over those bytes
	// (REQ-EVP-VERIFY-001). Any byte tamper lands here.
	reasonSignatureInvalid = "signature_invalid"
	// reasonSignatureKeyUnknown: the attestation key_id is not in the committed
	// public-key allowlist (REQ-EVP-KEYUNK-001). An empty allowlist rejects here.
	reasonSignatureKeyUnknown = "signature_key_unknown"
)

// EvalRegressionVerifyReasons exposes the three verify-stage reason literals to
// the CLI package so callers can assert them without importing unexported
// symbols. The map is rebuilt per call to avoid shared-state mutation.
func EvalRegressionVerifyReasons() map[string]string {
	return map[string]string{
		"unsigned":    reasonArtifactUnsigned,
		"invalid":     reasonSignatureInvalid,
		"key_unknown": reasonSignatureKeyUnknown,
	}
}

// @AX:WARN [AUTO] Verify-before-trust boundary — signature checked over RAW bytes; report blocked field is never read here on failure (INV-EVP-01/04).
// @AX:REASON: the function verifies ed25519 over reportBytes verbatim and never re-marshals; any byte tamper, unknown key_id, or malformed attestation fails closed before the caller can access the gate verdict.
// VerifyEvalRegressionArtifact verifies an ed25519 signature over the EXACT raw
// report bytes as passed in, against a trusted public-key allowlist selected by
// key_id. It NEVER re-marshals the report — signature and sha256 are checked
// over reportBytes verbatim so any on-disk byte tamper is detected.
//
// Fail-closed evaluation order (verify precedes trust; the report's blocked
// field is never read here):
//  1. absent/empty attestation      → artifact_unsigned
//  2. malformed attestation JSON     → signature_invalid
//  3. wrong schema_version/algorithm → signature_invalid
//  4. key_id not in allowlist/empty  → signature_key_unknown
//  5. sha256(reportBytes) mismatch   → signature_invalid
//  6. bad base64 / wrong sig size    → signature_invalid
//  7. wrong pubkey size / verify false → signature_invalid
//  8. otherwise                      → "", true
func VerifyEvalRegressionArtifact(reportBytes, attestationBytes []byte, trusted map[string]ed25519.PublicKey) (reason string, ok bool) {
	// 1. Absent or whitespace-only attestation is an unsigned artifact.
	if len(bytes.TrimSpace(attestationBytes)) == 0 {
		return reasonArtifactUnsigned, false
	}

	// 2. Strict decode: an unknown field or any syntax/type error is treated as
	// a malformed (untrusted) signature. DisallowUnknownFields prevents a
	// crafted sidecar from smuggling extra material past the verifier.
	dec := json.NewDecoder(bytes.NewReader(attestationBytes))
	dec.DisallowUnknownFields()
	var att EvalRegressionAttestationV1
	if err := dec.Decode(&att); err != nil {
		return reasonSignatureInvalid, false
	}

	// 3. Schema identity and algorithm must match exactly.
	if att.SchemaVersion != EvalRegressionAttestationSchemaV1 || att.Algorithm != "ed25519" {
		return reasonSignatureInvalid, false
	}

	// 4. Key selection. An empty key_id or one absent from the allowlist fails
	// closed as an unknown signer — this is the empty-allowlist reject path.
	if att.KeyID == "" {
		return reasonSignatureKeyUnknown, false
	}
	pub, present := trusted[att.KeyID]
	if !present {
		return reasonSignatureKeyUnknown, false
	}

	// 5. Recompute the digest over the raw report bytes and compare (case-
	// insensitive hex). A mismatch means the on-disk bytes were tampered.
	sum := sha256.Sum256(reportBytes)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), att.ReportSHA256) {
		return reasonSignatureInvalid, false
	}

	// 6. Decode the signature; a bad base64 or wrong signature size fails closed.
	sig, err := base64.StdEncoding.DecodeString(att.SignatureB64)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return reasonSignatureInvalid, false
	}

	// 7. Verify the signature over the raw report bytes. A malformed public key
	// or a failed verification fails closed.
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, reportBytes, sig) {
		return reasonSignatureInvalid, false
	}

	return "", true
}

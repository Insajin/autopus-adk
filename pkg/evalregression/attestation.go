package evalregression

import "crypto/ed25519"

// EvalRegressionAttestationSchemaV1 is the exact schema_version this consumer
// accepts for the out-of-band signature sidecar. It mirrors the Primary
// producer's attestation envelope (SPEC-EVAL-REGRESSION-PROV-001,
// REQ-EVP-SIGN-001). Any other value is treated as an invalid signature
// (reason signature_invalid) so a mismatched sidecar can never reach trust.
const EvalRegressionAttestationSchemaV1 = "eval_regression_attestation.v1"

// EvalRegressionAttestationSchemaV2 identifies the context-bound attestation
// envelope used by strict production consumers.
const EvalRegressionAttestationSchemaV2 = "eval_regression_attestation.v2"

// EvalRegressionAttestationV1 is the read-only consumer mirror of the producer's
// signature sidecar. It re-declares ONLY the six public fields the verifier
// reads; the producer single-sources signing. The json tags are the EXACT
// snake_case tags emitted by the Primary — do not rename or nest.
//
// It carries PUBLIC signature material only: the base64 ed25519 signature over
// the exact report bytes, the SHA-256 of those same bytes, and the key_id used
// to select a trusted public key. No private key material ever appears here.
type EvalRegressionAttestationV1 struct {
	SchemaVersion string `json:"schema_version"`
	KeyID         string `json:"key_id"`
	Algorithm     string `json:"algorithm"`
	SignatureB64  string `json:"signature_b64"`
	ReportSHA256  string `json:"report_sha256"`
	ProducedAt    string `json:"produced_at"`
}

// EvalRegressionAttestationV2 binds the report digest to an explicit trust
// lane, environment transition, source revision, and workspace scope.
type EvalRegressionAttestationV2 struct {
	SchemaVersion     string `json:"schema_version"`
	KeyID             string `json:"key_id"`
	Algorithm         string `json:"algorithm"`
	SignatureB64      string `json:"signature_b64"`
	ReportSHA256      string `json:"report_sha256"`
	ProducedAt        string `json:"produced_at"`
	TrustLane         string `json:"trust_lane"`
	SourceEnvironment string `json:"source_environment"`
	TargetEnvironment string `json:"target_environment"`
	SourceRevision    string `json:"source_revision"`
	WorkspaceScope    string `json:"workspace_scope"`
}

// EvalRegressionAttestationPolicyV2 defines the exact context a strict
// consumer expects. Every field is mandatory and compared without coercion.
type EvalRegressionAttestationPolicyV2 struct {
	ExpectedKeyID     string
	TrustLane         string
	SourceEnvironment string
	TargetEnvironment string
	SourceRevision    string
	WorkspaceScope    string
}

// @AX:NOTE [AUTO] Source of truth for trusted signers — PUBLIC keys only; empty allowlist is intentionally fail-closed (REQ-EVP-KEYUNK-001).
// evalRegressionPublicKeys is the committed public-key allowlist keyed by
// key_id. It is the SOURCE OF TRUTH for trusted signers on the consumer side.
//
// SECURITY INVARIANTS (SPEC-EVAL-REGRESSION-PROV-001):
//   - PUBLIC keys ONLY are committed here. Private/secret signing material is
//     NEVER placed in this map or anywhere in the repository — the producer
//     reads its private key from an env/secret source at sign time.
//   - MULTIPLE concurrent key_id entries are supported so an operator can run an
//     overlap-window key rotation (REQ-EVP-ROTATE-001): both the outgoing and
//     incoming public keys are allowlisted during the overlap, and the outgoing
//     entry is removed after the cutover.
//   - It is intentionally EMPTY today. The real production public key is added
//     by ops once LIVE-A emits real signed artifacts (Named Residual). An empty
//     allowlist is correctly fail-closed: any present artifact resolves no key
//     and is rejected as signature_key_unknown, and an absent attestation is
//     rejected as artifact_unsigned. There is no unsigned-accept path.
var evalRegressionPublicKeys = map[string]ed25519.PublicKey{}

// CommittedEvalRegressionPublicKeys returns a defensive copy of the committed
// public-key allowlist for the CLI production path. The copy prevents a caller
// from mutating the trusted set: each ed25519.PublicKey slice is cloned so the
// underlying key bytes cannot be swapped out through the returned map.
func CommittedEvalRegressionPublicKeys() map[string]ed25519.PublicKey {
	out := make(map[string]ed25519.PublicKey, len(evalRegressionPublicKeys))
	for keyID, pub := range evalRegressionPublicKeys {
		cloned := make(ed25519.PublicKey, len(pub))
		copy(cloned, pub)
		out[keyID] = cloned
	}
	return out
}

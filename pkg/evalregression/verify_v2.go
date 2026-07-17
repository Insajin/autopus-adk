package evalregression

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
)

const (
	evalRegressionAttestationV2Domain = "autopus.eval-regression.attestation.v2\x00"
	reasonAttestationPolicyInvalid    = "attestation_policy_invalid"
	reasonAttestationPolicyMismatch   = "attestation_policy_mismatch"
)

type evalRegressionAttestationStatementV2 struct {
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

type evalRegressionReportContextV2 struct {
	ProducedAt     string `json:"produced_at"`
	WorkspaceScope string `json:"workspace_scope"`
}

// EvalRegressionStrictVerifyReasons exposes stable strict-policy failure
// reasons without changing the v1 reason map contract.
func EvalRegressionStrictVerifyReasons() map[string]string {
	return map[string]string{
		"policy_invalid":  reasonAttestationPolicyInvalid,
		"policy_mismatch": reasonAttestationPolicyMismatch,
	}
}

// ValidateEvalRegressionAttestationPolicyV2 requires every strict expectation.
func ValidateEvalRegressionAttestationPolicyV2(policy EvalRegressionAttestationPolicyV2) (reason string, ok bool) {
	required := [...]string{
		policy.ExpectedKeyID,
		policy.TrustLane,
		policy.SourceEnvironment,
		policy.TargetEnvironment,
		policy.SourceRevision,
		policy.WorkspaceScope,
	}
	for _, value := range required {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return reasonAttestationPolicyInvalid, false
		}
	}
	return "", true
}

// VerifyEvalRegressionArtifactV2Strict verifies a context-bound v2 envelope.
// It preserves the v1 verifier as a separate compatibility API while making
// policy validation and exact policy matching mandatory for v2 callers.
func VerifyEvalRegressionArtifactV2Strict(reportBytes, attestationBytes []byte, trusted map[string]ed25519.PublicKey, policy EvalRegressionAttestationPolicyV2) (reason string, ok bool) {
	if reason, ok := ValidateEvalRegressionAttestationPolicyV2(policy); !ok {
		return reason, false
	}
	if len(bytes.TrimSpace(attestationBytes)) == 0 {
		return reasonArtifactUnsigned, false
	}

	att, ok := decodeEvalRegressionAttestationV2(attestationBytes)
	if !ok || att.SchemaVersion != EvalRegressionAttestationSchemaV2 || att.Algorithm != "ed25519" {
		return reasonSignatureInvalid, false
	}
	if !matchesEvalRegressionAttestationPolicyV2(att, policy) {
		return reasonAttestationPolicyMismatch, false
	}

	pub, present := trusted[att.KeyID]
	if !present {
		return reasonSignatureKeyUnknown, false
	}
	sum := sha256.Sum256(reportBytes)
	if att.ReportSHA256 != hex.EncodeToString(sum[:]) || strings.TrimSpace(att.ProducedAt) == "" {
		return reasonSignatureInvalid, false
	}
	sig, err := base64.StdEncoding.DecodeString(att.SignatureB64)
	if err != nil || len(sig) != ed25519.SignatureSize || len(pub) != ed25519.PublicKeySize {
		return reasonSignatureInvalid, false
	}
	message, err := evalRegressionAttestationV2Message(att)
	if err != nil || !ed25519.Verify(pub, message, sig) {
		return reasonSignatureInvalid, false
	}
	// Report context is intentionally read only after signature verification.
	// This catches a signer that attached valid expected context to report bytes
	// from a different workspace or production instant.
	if !matchesEvalRegressionReportContextV2(reportBytes, att) {
		return reasonAttestationPolicyMismatch, false
	}
	return "", true
}

func decodeEvalRegressionAttestationV2(data []byte) (EvalRegressionAttestationV2, bool) {
	var att EvalRegressionAttestationV2
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&att); err != nil {
		return att, false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return att, false
	}
	return att, true
}

func matchesEvalRegressionAttestationPolicyV2(att EvalRegressionAttestationV2, policy EvalRegressionAttestationPolicyV2) bool {
	return att.KeyID == policy.ExpectedKeyID &&
		att.TrustLane == policy.TrustLane &&
		att.SourceEnvironment == policy.SourceEnvironment &&
		att.TargetEnvironment == policy.TargetEnvironment &&
		att.SourceRevision == policy.SourceRevision &&
		att.WorkspaceScope == policy.WorkspaceScope
}

func matchesEvalRegressionReportContextV2(reportBytes []byte, att EvalRegressionAttestationV2) bool {
	var context evalRegressionReportContextV2
	dec := json.NewDecoder(bytes.NewReader(reportBytes))
	if err := dec.Decode(&context); err != nil {
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return false
	}
	return context.WorkspaceScope == att.WorkspaceScope && context.ProducedAt == att.ProducedAt
}

func evalRegressionAttestationV2Message(att EvalRegressionAttestationV2) ([]byte, error) {
	statement, err := json.Marshal(evalRegressionAttestationStatementV2{
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
		return nil, err
	}
	message := make([]byte, 0, len(evalRegressionAttestationV2Domain)+len(statement))
	message = append(message, evalRegressionAttestationV2Domain...)
	message = append(message, statement...)
	return message, nil
}

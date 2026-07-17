package cli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/evalregression"
)

type cliV2Statement struct {
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

func cliV2Policy() evalregression.EvalRegressionAttestationPolicyV2 {
	return evalregression.EvalRegressionAttestationPolicyV2{
		ExpectedKeyID:     "eval-cli-v2",
		TrustLane:         "production-promotion",
		SourceEnvironment: "staging",
		TargetEnvironment: "production",
		SourceRevision:    "0123456789abcdef0123456789abcdef01234567",
		WorkspaceScope:    "autopus-primary",
	}
}

func signArtifactV2(t *testing.T, artifactPath string, policy evalregression.EvalRegressionAttestationPolicyV2) (string, map[string]ed25519.PublicKey) {
	t.Helper()
	report, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	sum := sha256.Sum256(report)
	var reportContext struct {
		ProducedAt string `json:"produced_at"`
	}
	if err := json.Unmarshal(report, &reportContext); err != nil {
		t.Fatalf("decode report context: %v", err)
	}
	att := evalregression.EvalRegressionAttestationV2{
		SchemaVersion:     evalregression.EvalRegressionAttestationSchemaV2,
		KeyID:             policy.ExpectedKeyID,
		Algorithm:         "ed25519",
		ReportSHA256:      hex.EncodeToString(sum[:]),
		ProducedAt:        reportContext.ProducedAt,
		TrustLane:         policy.TrustLane,
		SourceEnvironment: policy.SourceEnvironment,
		TargetEnvironment: policy.TargetEnvironment,
		SourceRevision:    policy.SourceRevision,
		WorkspaceScope:    policy.WorkspaceScope,
	}
	statement, err := json.Marshal(cliV2Statement{
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
		t.Fatalf("marshal statement: %v", err)
	}
	message := append([]byte("autopus.eval-regression.attestation.v2\x00"), statement...)
	att.SignatureB64 = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message))
	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("marshal attestation: %v", err)
	}
	attPath := deriveEvalRegressionAttestationPath(artifactPath)
	if err := os.WriteFile(attPath, data, 0o600); err != nil {
		t.Fatalf("write attestation: %v", err)
	}
	return attPath, map[string]ed25519.PublicKey{policy.ExpectedKeyID: pub}
}

func TestCheckEvalRegressionStrictV2PassesAndRejectsMismatch(t *testing.T) {
	path := writeArtifact(t, `{
		"schema_version":"eval_regression_report.v1",
		"blocked":false,
		"attributed_version":"candidate",
		"produced_at":"2026-07-03T12:00:00Z",
		"workspace_scope":"autopus-primary",
		"raw_payload_present":false
	}`)
	policy := cliV2Policy()
	attPath, trusted := signArtifactV2(t, path, policy)

	var out bytes.Buffer
	if pass := checkEvalRegressionStrict("", path, attPath, 24*time.Hour, fixedNow, trusted, policy, &out, false, false); !pass {
		t.Fatalf("strict v2 control rejected: %q", out.String())
	}

	out.Reset()
	mismatch := policy
	mismatch.WorkspaceScope = "other-workspace"
	if pass := checkEvalRegressionStrict("", path, attPath, 24*time.Hour, fixedNow, trusted, mismatch, &out, false, false); pass {
		t.Fatalf("strict mismatch unexpectedly passed")
	}
	if !strings.Contains(out.String(), "attestation_policy_mismatch") {
		t.Fatalf("expected stable mismatch reason, got %q", out.String())
	}
}

func TestCheckEvalRegressionStrictInvalidPolicyCannotUseWarnOnly(t *testing.T) {
	var out bytes.Buffer
	pass := checkEvalRegressionStrict(
		"",
		"missing.json",
		"missing.attestation.json",
		24*time.Hour,
		fixedNow,
		nil,
		evalregression.EvalRegressionAttestationPolicyV2{},
		&out,
		false,
		true,
	)
	if pass {
		t.Fatalf("invalid trust policy must fail even with warn-only")
	}
	if !strings.Contains(out.String(), "attestation_policy_invalid") {
		t.Fatalf("expected stable invalid-policy reason, got %q", out.String())
	}
}

func TestCheckCmdEvalRegressionRequiresEveryTrustFlag(t *testing.T) {
	flags := map[string]string{
		"--eval-regression-expected-key-id":             "eval-cli-v2",
		"--eval-regression-expected-trust-lane":         "production-promotion",
		"--eval-regression-expected-source-environment": "staging",
		"--eval-regression-expected-target-environment": "production",
		"--eval-regression-expected-source-revision":    "0123456789abcdef0123456789abcdef01234567",
		"--eval-regression-expected-workspace-scope":    "autopus-primary",
	}
	for omitted := range flags {
		t.Run(omitted, func(t *testing.T) {
			cmd := newCheckCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			args := []string{
				"--eval-regression",
				"--eval-regression-artifact", "missing.json",
				"--warn-only",
			}
			for flag, value := range flags {
				if flag != omitted {
					args = append(args, flag, value)
				}
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("missing %s must fail closed", omitted)
			}
			if !strings.Contains(out.String(), "attestation_policy_invalid") {
				t.Fatalf("missing %s: expected stable reason, got %q", omitted, out.String())
			}
		})
	}
}

func TestCheckCmdEvalRegressionRegistersV2TrustFlags(t *testing.T) {
	cmd := newCheckCmd()
	for _, name := range []string{
		"eval-regression-expected-key-id",
		"eval-regression-expected-trust-lane",
		"eval-regression-expected-source-environment",
		"eval-regression-expected-target-environment",
		"eval-regression-expected-source-revision",
		"eval-regression-expected-workspace-scope",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s", name)
		}
	}
}

func TestCheckCmdEvalRegressionValidatesPolicyBeforeNamedGate(t *testing.T) {
	cmd := newCheckCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--eval-regression", "--gate", "phase2", "--warn-only"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("named gate bypassed required eval-regression policy")
	}
	if !strings.Contains(out.String(), "attestation_policy_invalid") {
		t.Fatalf("expected stable invalid-policy reason, got %q", out.String())
	}
}

func TestCheckCmdEvalRegressionRejectsNamedGateBypass(t *testing.T) {
	cmd := newCheckCmd()
	cmd.SetArgs([]string{
		"--eval-regression",
		"--gate", "phase2",
		"--eval-regression-expected-key-id", "eval-cli-v2",
		"--eval-regression-expected-trust-lane", "production-promotion",
		"--eval-regression-expected-source-environment", "staging",
		"--eval-regression-expected-target-environment", "production",
		"--eval-regression-expected-source-revision", "0123456789abcdef0123456789abcdef01234567",
		"--eval-regression-expected-workspace-scope", "autopus-primary",
	})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("named gate bypass was not rejected: %v", err)
	}
}

func TestCheckCmdEvalRegressionProductionPathRejectsV1Attestation(t *testing.T) {
	path := writeArtifact(t, `{
		"schema_version":"eval_regression_report.v1",
		"blocked":false,
		"attributed_version":"candidate",
		"produced_at":"2026-07-03T12:00:00Z",
		"workspace_scope":"autopus-primary",
		"raw_payload_present":false
	}`)
	attPath, _ := signArtifact(t, path)
	cmd := newCheckCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--eval-regression",
		"--eval-regression-artifact", path,
		"--eval-regression-attestation", attPath,
		"--eval-regression-expected-key-id", "evp-cli-1",
		"--eval-regression-expected-trust-lane", "production-promotion",
		"--eval-regression-expected-source-environment", "staging",
		"--eval-regression-expected-target-environment", "production",
		"--eval-regression-expected-source-revision", "0123456789abcdef0123456789abcdef01234567",
		"--eval-regression-expected-workspace-scope", "autopus-primary",
	})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("production path accepted a v1 attestation")
	}
	if !strings.Contains(out.String(), "signature_invalid") {
		t.Fatalf("expected strict v2 rejection, got %q", out.String())
	}
}

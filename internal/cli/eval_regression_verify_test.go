package cli

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"strings"
	"testing"
	"time"
)

// This file holds the CLI-level signature-verify oracles (S2/S3/S4) for
// SPEC-EVAL-REGRESSION-PROV-001. They exercise checkEvalRegression end to end so
// the verify-before-trust wiring and its exit-code proxy (the returned bool) are
// observable. The shared writeArtifact/signArtifact helpers and fixedNow live in
// eval_regression_test.go (same package).

// S3 (CLI) — a present report with an ABSENT attestation fails closed with
// reason artifact_unsigned (exit 1). No unsigned-accept path.
func TestCheckEvalRegressionUnsignedFailsClosed(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": false,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"raw_payload_present": false
	}`
	path := writeArtifact(t, body)
	attPath := deriveEvalRegressionAttestationPath(path) // never written

	var out bytes.Buffer
	pass := checkEvalRegression("", path, attPath, 24*time.Hour, fixedNow, map[string]ed25519.PublicKey{}, &out, false, false)
	if pass {
		t.Fatalf("S3: expected fail (false) for an unsigned artifact, got pass")
	}
	if !strings.Contains(out.String(), "artifact_unsigned") {
		t.Fatalf("S3: expected reason artifact_unsigned in output, got %q", out.String())
	}
}

// S4 (CLI) — a validly-signed artifact whose key_id is NOT in the trusted map
// fails closed with reason signature_key_unknown (exit 1). The empty trusted map
// simulates the production allowlist before ops adds the real key.
func TestCheckEvalRegressionUnknownKeyFailsClosed(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": false,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"raw_payload_present": false
	}`
	path := writeArtifact(t, body)
	attPath, _ := signArtifact(t, path) // signed, but we DROP the trusted map

	var out bytes.Buffer
	pass := checkEvalRegression("", path, attPath, 24*time.Hour, fixedNow, map[string]ed25519.PublicKey{}, &out, false, false)
	if pass {
		t.Fatalf("S4: expected fail (false) for an unknown key_id, got pass")
	}
	if !strings.Contains(out.String(), "signature_key_unknown") {
		t.Fatalf("S4: expected reason signature_key_unknown in output, got %q", out.String())
	}
}

// S2 (CLI, core tamper) — a validly-signed blocked:true report whose on-disk
// bytes are then mutated to flip blocked true→false (keeping the old signature
// and old report_sha256) fails closed with reason signature_invalid. The mutated
// blocked value is never read because verify precedes decode.
func TestCheckEvalRegressionTamperFailsClosed(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": true,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"raw_payload_present": false
	}`
	path := writeArtifact(t, body)
	attPath, trusted := signArtifact(t, path) // signs the blocked:true bytes

	// Attacker flips blocked true→false on disk, keeping the old attestation.
	mutated := strings.Replace(body, `"blocked": true`, `"blocked": false`, 1)
	if mutated == body {
		t.Fatalf("S2: test setup failed to mutate blocked")
	}
	if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
		t.Fatalf("S2: rewrite mutated artifact: %v", err)
	}

	var out bytes.Buffer
	pass := checkEvalRegression("", path, attPath, 24*time.Hour, fixedNow, trusted, &out, false, false)
	if pass {
		t.Fatalf("S2: expected fail (false) for tampered bytes, got pass")
	}
	if !strings.Contains(out.String(), "signature_invalid") {
		t.Fatalf("S2: expected reason signature_invalid in output, got %q", out.String())
	}
}

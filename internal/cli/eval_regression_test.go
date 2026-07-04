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
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedNow is the injected clock used across the deterministic oracles so the
// staleness axis never reads the wall clock.
var fixedNow = time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

func writeArtifact(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "eval_regression_report.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	return path
}

// signArtifact co-writes a VALID eval_regression_attestation.v1 sidecar next to
// the report artifact using a fresh test-only ed25519 keypair, and returns the
// derived sidecar path plus a trusted allowlist containing the public key. It is
// the fixture that lets the preserved decode/gate chain be exercised under
// mandatory verify-before-trust.
func signArtifact(t *testing.T, artifactPath string) (string, map[string]ed25519.PublicKey) {
	t.Helper()
	reportBytes, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact for signing: %v", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test keypair: %v", err)
	}
	const keyID = "evp-cli-1"
	sum := sha256.Sum256(reportBytes)
	att := map[string]string{
		"schema_version": "eval_regression_attestation.v1",
		"key_id":         keyID,
		"algorithm":      "ed25519",
		"signature_b64":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, reportBytes)),
		"report_sha256":  hex.EncodeToString(sum[:]),
		"produced_at":    "2026-07-03T11:00:00Z",
	}
	attBytes, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("marshal attestation: %v", err)
	}
	attPath := deriveEvalRegressionAttestationPath(artifactPath)
	if err := os.WriteFile(attPath, attBytes, 0o600); err != nil {
		t.Fatalf("write attestation: %v", err)
	}
	return attPath, map[string]ed25519.PublicKey{keyID: pub}
}

// G3 — a missing artifact file fails closed with reason artifact_missing and a
// non-zero (fail) verdict. Missing PRECEDES verify: no attestation exists yet
// the reason is artifact_missing (not artifact_unsigned).
func TestCheckEvalRegressionMissingArtifactFailsClosed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")

	var out bytes.Buffer
	pass := checkEvalRegression("", missing, missing+".attestation.json", 24*time.Hour, fixedNow, nil, &out, false, false)

	if pass {
		t.Fatalf("G3: expected fail (false) for a missing artifact, got pass")
	}
	if !strings.Contains(out.String(), reasonArtifactMissing) {
		t.Fatalf("G3: expected reason %q in output, got %q", reasonArtifactMissing, out.String())
	}
}

// G3 (loader) — the loader itself maps an absent file to reason artifact_missing.
func TestLoadEvalRegressionReportMissingReason(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.json")
	_, reason, err := loadEvalRegressionReport(missing)
	if err == nil {
		t.Fatalf("expected error for missing artifact")
	}
	if reason != reasonArtifactMissing {
		t.Fatalf("expected reason %q, got %q", reasonArtifactMissing, reason)
	}
}

// unknown-field — an artifact carrying a field outside the consumer schema is
// rejected with reason artifact_unsafe (strict decode, not silently dropped).
func TestLoadEvalRegressionReportUnknownFieldIsUnsafe(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": false,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"raw_payload_present": false,
		"leaked_raw_prompt": "SECRET_SHOULD_NOT_BE_DROPPED"
	}`
	path := writeArtifact(t, body)

	_, reason, err := loadEvalRegressionReport(path)
	if err == nil {
		t.Fatalf("unknown-field: expected strict-decode error, got nil")
	}
	if reason != reasonArtifactUnsafe {
		t.Fatalf("unknown-field: expected reason %q, got %q", reasonArtifactUnsafe, reason)
	}
}

// unknown-field (branch) — checkEvalRegression fails closed on an unknown field
// AFTER a valid signature verify (verify precedes decode; the sidecar signs the
// exact bytes including the extra field, so verify passes and decode rejects).
func TestCheckEvalRegressionUnknownFieldFailsClosed(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": false,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"extra_field": true
	}`
	path := writeArtifact(t, body)
	attPath, trusted := signArtifact(t, path)

	var out bytes.Buffer
	pass := checkEvalRegression("", path, attPath, 24*time.Hour, fixedNow, trusted, &out, false, false)
	if pass {
		t.Fatalf("unknown-field: expected fail (false), got pass")
	}
	if !strings.Contains(out.String(), reasonArtifactUnsafe) {
		t.Fatalf("unknown-field: expected reason %q in output, got %q", reasonArtifactUnsafe, out.String())
	}
}

// trailing-data — a valid object followed by extra bytes is malformed input and
// is rejected with reason artifact_invalid (fail closed), not silently truncated
// to the leading object.
func TestLoadEvalRegressionReportTrailingDataIsInvalid(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": false,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"raw_payload_present": false
	}{"x":1}`
	path := writeArtifact(t, body)

	_, reason, err := loadEvalRegressionReport(path)
	if err == nil {
		t.Fatalf("trailing-data: expected error, got nil")
	}
	if reason != reasonArtifactInvalid {
		t.Fatalf("trailing-data: expected reason %q, got %q", reasonArtifactInvalid, reason)
	}
}

// G4 (CLI) — malformed JSON bytes fail closed through checkEvalRegression with
// reason artifact_invalid. The sidecar signs the exact malformed bytes so verify
// passes and the failure surfaces at strict-decode.
func TestCheckEvalRegressionBadJSONFailsClosed(t *testing.T) {
	path := writeArtifact(t, `{not json`)
	attPath, trusted := signArtifact(t, path)

	var out bytes.Buffer
	pass := checkEvalRegression("", path, attPath, 24*time.Hour, fixedNow, trusted, &out, false, false)
	if pass {
		t.Fatalf("G4: expected fail (false) for malformed JSON, got pass")
	}
	if !strings.Contains(out.String(), reasonArtifactInvalid) {
		t.Fatalf("G4: expected reason %q in output, got %q", reasonArtifactInvalid, out.String())
	}
}

// G7 — a blocked artifact with --warn-only prints the blocked verdict yet the
// gate exits zero (advisory), so the function returns true.
func TestCheckEvalRegressionWarnOnlyBlockedAdvisory(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": true,
		"regression_delta": 0.30,
		"threshold_value": 0.10,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"raw_payload_present": false
	}`
	path := writeArtifact(t, body)
	attPath, trusted := signArtifact(t, path)

	var out bytes.Buffer
	pass := checkEvalRegression("", path, attPath, 24*time.Hour, fixedNow, trusted, &out, false, true)
	if !pass {
		t.Fatalf("G7: expected pass (true) in warn-only advisory mode, got fail")
	}
	if !strings.Contains(out.String(), "regression_blocked") {
		t.Fatalf("G7: expected blocked verdict printed, got %q", out.String())
	}
	if !strings.Contains(out.String(), "candidate") {
		t.Fatalf("G7: expected sanitized attributed version 'candidate' in output, got %q", out.String())
	}
}

// S5 (CLI) — a validly-signed, allowlisted, fresh blocked artifact WITHOUT
// warn-only fails the check (exit 1) with reason regression_blocked.
func TestCheckEvalRegressionBlockedFailsWithoutWarnOnly(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": true,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"raw_payload_present": false
	}`
	path := writeArtifact(t, body)
	attPath, trusted := signArtifact(t, path)

	var out bytes.Buffer
	pass := checkEvalRegression("", path, attPath, 24*time.Hour, fixedNow, trusted, &out, false, false)
	if pass {
		t.Fatalf("expected fail (false) for a blocked artifact without warn-only")
	}
	if !strings.Contains(out.String(), "regression_blocked") {
		t.Fatalf("expected reason regression_blocked in output, got %q", out.String())
	}
}

// S1 (CLI) — a validly-signed, allowlisted, fresh non-blocked control artifact
// passes the check with reason ok (exit 0).
func TestCheckEvalRegressionControlPasses(t *testing.T) {
	body := `{
		"schema_version": "eval_regression_report.v1",
		"blocked": false,
		"regression_delta": 0.04,
		"attributed_version": "candidate",
		"produced_at": "2026-07-03T12:00:00Z",
		"raw_payload_present": false
	}`
	path := writeArtifact(t, body)
	attPath, trusted := signArtifact(t, path)

	var out bytes.Buffer
	pass := checkEvalRegression("", path, attPath, 24*time.Hour, fixedNow, trusted, &out, false, false)
	if !pass {
		t.Fatalf("expected pass (true) for a non-blocked control artifact")
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatalf("expected reason ok in output, got %q", out.String())
	}
}

// G8 — the committed PR workflow wires the gate without --warn-only.
func TestEvalRegressionWorkflowWiredWithoutWarnOnly(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "eval-regression-gate.yml"))
	if err != nil {
		t.Skipf("G8: workflow not present (owned by workflow executor): %v", err)
	}
	yaml := string(data)

	if !strings.Contains(yaml, "pull_request") {
		t.Fatalf("G8: workflow on: triggers must include pull_request")
	}

	var runLine string
	for _, line := range strings.Split(yaml, "\n") {
		if strings.Contains(line, "auto check --eval-regression") {
			runLine = line
			break
		}
	}
	if runLine == "" {
		t.Fatalf("G8: no run line contains 'auto check --eval-regression'")
	}
	if !strings.Contains(runLine, "--eval-regression-artifact") {
		t.Fatalf("G8: eval-regression run line must pass --eval-regression-artifact, got %q", runLine)
	}
	if strings.Contains(runLine, "--warn-only") {
		t.Fatalf("G8: the auto-check step must NOT contain --warn-only, got %q", runLine)
	}
}

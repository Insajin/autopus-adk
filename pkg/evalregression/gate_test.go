package evalregression

import (
	"strings"
	"testing"
	"time"
)

// now is the fixed injected clock for deterministic freshness oracles.
var now = time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

// fresh returns a produced_at 1h before now (well inside a 24h window).
func fresh() time.Time { return now.Add(-1 * time.Hour) }

// validBlocked builds a schema-valid, safe, fresh, blocked report with a
// conforming attributed version.
func validBlocked() EvalRegressionReportV1 {
	return EvalRegressionReportV1{
		SchemaVersion:     EvalRegressionReportSchemaV1,
		Blocked:           true,
		RegressionDelta:   0.30,
		AttributedVersion: "candidate",
		ThresholdValue:    0.10,
		ProducedAt:        fresh(),
		RawPayloadPresent: false,
	}
}

const window = 24 * time.Hour

// rawPayloadSubstrings are strings that must never appear in a gate reason;
// they stand in for provider/customer/prompt payload leakage.
var rawPayloadSubstrings = []string{"provider", "customer", "prompt", "secret", "token", "/"}

func assertNoRawPayload(t *testing.T, reason string) {
	t.Helper()
	for _, s := range rawPayloadSubstrings {
		if strings.Contains(reason, s) {
			t.Errorf("reason %q leaked forbidden substring %q", reason, s)
		}
	}
}

// G1: blocked verdict fails and names the candidate version, leaks nothing.
func TestG1_BlockedNamesCandidate(t *testing.T) {
	d := EvaluateEvalRegressionGate(validBlocked(), now, window)
	if !d.Blocked {
		t.Errorf("Blocked = false, want true")
	}
	if d.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", d.ExitCode)
	}
	if d.Reason != "regression_blocked" {
		t.Errorf("Reason = %q, want regression_blocked", d.Reason)
	}
	if d.AttributedVersion != "candidate" {
		t.Errorf("AttributedVersion = %q, want candidate", d.AttributedVersion)
	}
	full := d.Reason + " " + d.AttributedVersion
	if !strings.Contains(full, "candidate") {
		t.Errorf("emitted output %q does not name candidate version", full)
	}
	assertNoRawPayload(t, d.Reason)
}

// G2: non-blocked control passes with reason ok, exit 0.
func TestG2_ControlPasses(t *testing.T) {
	r := validBlocked()
	r.Blocked = false
	r.RegressionDelta = 0.04
	d := EvaluateEvalRegressionGate(r, now, window)
	if d.Blocked {
		t.Errorf("Blocked = true, want false")
	}
	if d.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", d.ExitCode)
	}
	if d.Reason != "ok" {
		t.Errorf("Reason = %q, want ok", d.Reason)
	}
}

// G4: wrong schema_version fails closed with artifact_invalid.
func TestG4_WrongSchemaInvalid(t *testing.T) {
	r := validBlocked()
	r.SchemaVersion = "eval_regression_report.v2"
	d := EvaluateEvalRegressionGate(r, now, window)
	if d.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", d.ExitCode)
	}
	if d.Reason != "artifact_invalid" {
		t.Errorf("Reason = %q, want artifact_invalid", d.Reason)
	}
}

// G5: stale artifact fails closed; fresh one passes the freshness axis; the
// window boundary is exact (age == window is fresh, age > window is stale).
func TestG5_Staleness(t *testing.T) {
	stale := validBlocked()
	stale.ProducedAt = now.Add(-48 * time.Hour)
	if d := EvaluateEvalRegressionGate(stale, now, window); d.Reason != "artifact_stale" || d.ExitCode != 1 {
		t.Errorf("48h/24h: got reason %q exit %d, want artifact_stale exit 1", d.Reason, d.ExitCode)
	}

	freshR := validBlocked()
	freshR.Blocked = false
	freshR.ProducedAt = now.Add(-1 * time.Hour)
	if d := EvaluateEvalRegressionGate(freshR, now, window); d.Reason == "artifact_stale" {
		t.Errorf("1h/24h: got artifact_stale, want fresh pass")
	}

	// Boundary: age exactly equal to window is fresh, not stale.
	boundary := validBlocked()
	boundary.Blocked = false
	boundary.ProducedAt = now.Add(-window)
	if d := EvaluateEvalRegressionGate(boundary, now, window); d.Reason == "artifact_stale" {
		t.Errorf("age==window: got artifact_stale, want fresh")
	}

	// Just over the boundary is stale.
	over := validBlocked()
	over.ProducedAt = now.Add(-window - time.Nanosecond)
	if d := EvaluateEvalRegressionGate(over, now, window); d.Reason != "artifact_stale" {
		t.Errorf("age>window: got %q, want artifact_stale", d.Reason)
	}
}

// TestFutureDatedProducedAtIsStale: a produced_at that leads the gate clock by
// more than the clock-skew tolerance is rejected as artifact_stale, exit 1, even
// with a generous window. A produced_at within the small skew stays fresh.
func TestFutureDatedProducedAtIsStale(t *testing.T) {
	future := validBlocked()
	future.Blocked = false
	future.ProducedAt = now.Add(1 * time.Hour) // 1h ahead of now
	if d := EvaluateEvalRegressionGate(future, now, window); d.Reason != "artifact_stale" || d.ExitCode != 1 {
		t.Errorf("future+1h/24h: got reason %q exit %d, want artifact_stale exit 1", d.Reason, d.ExitCode)
	}

	// Within the skew tolerance the artifact is still treated as fresh.
	skewOK := validBlocked()
	skewOK.Blocked = false
	skewOK.ProducedAt = now.Add(1 * time.Minute)
	if d := EvaluateEvalRegressionGate(skewOK, now, window); d.Reason == "artifact_stale" {
		t.Errorf("future+1m within skew: got artifact_stale, want fresh pass")
	}
}

// G6: RawPayloadPresent=true is rejected as artifact_unsafe.
func TestG6_RawPayloadUnsafe(t *testing.T) {
	r := validBlocked()
	r.RawPayloadPresent = true
	d := EvaluateEvalRegressionGate(r, now, window)
	if d.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", d.ExitCode)
	}
	if d.Reason != "artifact_unsafe" {
		t.Errorf("Reason = %q, want artifact_unsafe", d.Reason)
	}
	assertNoRawPayload(t, d.Reason)
}

// G9: non-conforming attributed versions fail closed and are never echoed.
func TestG9_NonConformingVersionNeverEchoed(t *testing.T) {
	tokenLike := "ghp_EXAMPLE_NOT_A_REAL_TOKEN"
	longBlob := strings.Repeat("a", 200)
	rejects := []string{
		tokenLike,
		"/etc/passwd",
		"/abs/path/example",
		"has space",
		"UPPER",
		"with\nnewline",
		"with\x00ctrl",
		longBlob,
	}
	for _, v := range rejects {
		r := validBlocked()
		r.AttributedVersion = v
		d := EvaluateEvalRegressionGate(r, now, window)
		if d.ExitCode != 1 {
			t.Errorf("version %q: ExitCode = %d, want 1", v, d.ExitCode)
		}
		if d.Reason != "artifact_unsafe" {
			t.Errorf("version %q: Reason = %q, want artifact_unsafe", v, d.Reason)
		}
		if strings.Contains(d.Reason, v) || strings.Contains(d.AttributedVersion, v) {
			t.Errorf("version %q leaked into output (reason=%q attr=%q)", v, d.Reason, d.AttributedVersion)
		}
		if d.AttributedVersion != "" {
			t.Errorf("version %q: AttributedVersion echoed %q, want empty", v, d.AttributedVersion)
		}
	}
}

// TestSanitizeAttributedVersion pins the version-shape allowlist directly:
// semver, git SHA, and lowercase identifiers accept; everything else rejects.
func TestSanitizeAttributedVersion(t *testing.T) {
	sha40 := "0123456789abcdef0123456789abcdef01234567"
	accepts := []string{
		"candidate",                   // lowercase identifier
		"v1.2.3",                      // semver with v prefix
		"1.2.3",                       // semver no prefix
		"1",                           // single-segment semver
		"1.2.3.4",                     // four-segment
		"v1.2.3-rc.1",                 // prerelease
		sha40,                         // 40-hex git SHA
		"abc1234",                     // 7-hex short SHA
		"a",                           // single-char identifier
		"a" + strings.Repeat("z", 31), // 32-char identifier (a + 31, at the cap)
	}
	for _, v := range accepts {
		out, ok := sanitizeAttributedVersion(v)
		if !ok {
			t.Errorf("sanitize(%q) rejected, want accept", v)
		}
		if out != v {
			t.Errorf("sanitize(%q) = %q, want passthrough", v, out)
		}
	}

	rejects := []string{
		"ghp_EXAMPLE_NOT_A_REAL_TOKEN",
		"/etc/passwd",
		"UPPER",
		"has space",
		"with\nnewline",
		"with\x00ctrl",
		"",
		"a" + strings.Repeat("z", 32), // 33-char identifier (over the 32 cap)
		strings.Repeat("a", 200),
		"under_score",
	}
	for _, v := range rejects {
		out, ok := sanitizeAttributedVersion(v)
		if ok {
			t.Errorf("sanitize(%q) accepted, want reject", v)
		}
		if out != "" {
			t.Errorf("sanitize(%q) returned %q on reject, want empty", v, out)
		}
	}
}

// TestEvaluationOrder confirms fail-closed checks run BEFORE the blocked read:
// an unsafe artifact that is also blocked reports the unsafe reason, never the
// pass path, and an invalid schema wins even over an unsafe payload.
func TestEvaluationOrder(t *testing.T) {
	// unsafe + blocked -> unsafe (safety checked before verdict).
	unsafe := validBlocked()
	unsafe.RawPayloadPresent = true
	if d := EvaluateEvalRegressionGate(unsafe, now, window); d.Reason != "artifact_unsafe" {
		t.Errorf("unsafe+blocked: reason %q, want artifact_unsafe", d.Reason)
	}

	// invalid schema wins over raw payload.
	bad := validBlocked()
	bad.SchemaVersion = "x"
	bad.RawPayloadPresent = true
	if d := EvaluateEvalRegressionGate(bad, now, window); d.Reason != "artifact_invalid" {
		t.Errorf("invalid+unsafe: reason %q, want artifact_invalid", d.Reason)
	}
}

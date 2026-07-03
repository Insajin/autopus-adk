package evalregression

import (
	"regexp"
	"time"
)

// Reason codes emitted by the gate. These are stable machine-readable literals
// asserted by the deterministic oracles (G1–G9). Never interpolate untrusted
// artifact data into a reason beyond an allowlist-sanitized attributed version.
const (
	reasonOK      = "ok"
	reasonBlocked = "regression_blocked"
	reasonInvalid = "artifact_invalid"
	reasonStale   = "artifact_stale"
	reasonUnsafe  = "artifact_unsafe"
)

// GateDecision is the deterministic verdict the gate returns. The Reason field
// is a machine-readable code ([NEW] relative to workflow.GateResult, which
// carries no reason). ExitCode mirrors the exit-code discipline of
// pkg/workflow/gate.go: 0 passes, 1 fails the PR check.
type GateDecision struct {
	Blocked           bool
	ExitCode          int
	Reason            string
	AttributedVersion string
}

// Version-shape allowlist (REQ-ECI-SANITIZE-001). Union of three anchored
// shapes; the first match accepts. Anchoring structurally excludes path
// separators, control chars, whitespace, and underscores (the semver prerelease
// suffix permits A-Za-z), so a token-like or path-like string matches none and
// fails closed.
var (
	semverShape = regexp.MustCompile(`^v?[0-9]+(\.[0-9]+){0,3}(-[0-9A-Za-z.]+)?$`)
	gitSHAShape = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	identShape  = regexp.MustCompile(`^[a-z][a-z0-9]{0,31}$`)
)

// maxAttributedVersionLen caps input length before regex evaluation as a cheap
// upfront guard; a conforming version never approaches 128 characters.
const maxAttributedVersionLen = 128

// maxClockSkew bounds how far a producer's produced_at may lead the gate clock
// before the artifact is rejected as stale. A clearly future-dated artifact is
// not trustworthy: it defeats the freshness axis (a large future timestamp can
// never age out of any window), so a lead greater than this small tolerance
// fails closed as artifact_stale (REQ-ECI-STALE-001).
const maxClockSkew = 5 * time.Minute

// sanitizeAttributedVersion returns (v, true) iff v matches one of the
// conservative version shapes (semver, git SHA, lowercase identifier ≤32). A
// non-conforming value returns ("", false) so a rejected version is never
// echoed. Over-rejection is the safe fail-closed direction: a charset allowlist
// cannot separate a secret from a version, so the shape allowlist is used.
func sanitizeAttributedVersion(v string) (string, bool) {
	if v == "" || len(v) > maxAttributedVersionLen {
		return "", false
	}
	if semverShape.MatchString(v) || gitSHAShape.MatchString(v) || identShape.MatchString(v) {
		return v, true
	}
	return "", false
}

// EvaluateEvalRegressionGate is a pure function of the artifact, an injected
// clock, and a freshness window (no I/O, deterministic — REQ-ECI-DETERM-001).
//
// Evaluation order is fail-closed: schema, raw-payload, and version-shape checks
// all run BEFORE the Blocked verdict is read, so an untrusted artifact can never
// reach the pass path. Any failed check returns exit 1; only a fully trusted,
// fresh, non-blocked artifact returns exit 0.
func EvaluateEvalRegressionGate(report EvalRegressionReportV1, now time.Time, freshness time.Duration) GateDecision {
	// 1. Schema identity (REQ-ECI-INVALID-001).
	if report.SchemaVersion != EvalRegressionReportSchemaV1 {
		return GateDecision{Blocked: true, ExitCode: 1, Reason: reasonInvalid}
	}

	// 2. Raw payload rejection (REQ-ECI-REDACT-001).
	if report.RawPayloadPresent {
		return GateDecision{Blocked: true, ExitCode: 1, Reason: reasonUnsafe}
	}

	// 3. Attributed-version shape allowlist (REQ-ECI-SANITIZE-001). Checked
	// before the verdict read so a non-conforming version fails closed and is
	// never echoed, even for a non-blocked artifact.
	safeVersion, ok := sanitizeAttributedVersion(report.AttributedVersion)
	if !ok {
		return GateDecision{Blocked: true, ExitCode: 1, Reason: reasonUnsafe}
	}

	// 4. Freshness (REQ-ECI-STALE-001). Boundary: age == window is fresh.
	if now.Sub(report.ProducedAt) > freshness {
		return GateDecision{Blocked: true, ExitCode: 1, Reason: reasonStale, AttributedVersion: safeVersion}
	}

	// 4b. Future-dating guard (REQ-ECI-STALE-001). A produced_at leading the gate
	// clock by more than maxClockSkew is not trustworthy and is treated as stale.
	if report.ProducedAt.Sub(now) > maxClockSkew {
		return GateDecision{Blocked: true, ExitCode: 1, Reason: reasonStale, AttributedVersion: safeVersion}
	}

	// 5. Verdict surfacing (REQ-ECI-GATE-001 / REQ-ECI-PASS-001).
	if report.Blocked {
		return GateDecision{Blocked: true, ExitCode: 1, Reason: reasonBlocked, AttributedVersion: safeVersion}
	}

	return GateDecision{Blocked: false, ExitCode: 0, Reason: reasonOK, AttributedVersion: safeVersion}
}

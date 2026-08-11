package execplane_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/execplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntitlementParityWithoutEvidenceStaysUnverified covers S8 / REQ-009 and
// is the sharpest edge of REQ-004: entitlement parity transfers evidence
// between accounts, it does not create any. A provider that holds nothing —
// no catalog, no reference judgement — has nothing for parity to carry, so the
// cleanest possible grade match still leaves the tier unverified, and the
// reason says so rather than reusing the wording of a trusted comparison.
func TestEntitlementParityWithoutEvidenceStaysUnverified(t *testing.T) {
	t.Parallel()

	resolution := fixtureDeterminedAccount(execplane.ProviderClaude, "execution@example.test")
	matched := execplane.Entitlement{Grade: "claude_max", Source: "host-cli@example.test"}
	verdict, _ := execplane.CompareEntitlement(matched, matched)
	require.Equal(t, execplane.VerdictTrusted, verdict,
		"the fixture must be a clean parity or it proves nothing about evidence")

	// The zero value and the explicit "none" are the same claim written two
	// ways, and a gate that only checked one of them would let an unset field
	// through as if it were evidence.
	for _, evidence := range []execplane.EvidenceKind{"", execplane.EvidenceNone} {
		receipt := execplane.Evaluate(
			fixtureTierRequest(execplane.ProviderClaude, "ultra", "claude-opus-5", evidence),
			resolution, matched, matched, time.Now(),
		)

		assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus,
			"evidence %q: parity alone must not promote a tier to verified", evidence)
		assert.Equal(t, "execution@example.test", receipt.ExecutionAccount,
			"evidence %q: the executor is still known; only the evidence is missing", evidence)
		assert.NotEmpty(t, receipt.ResolutionReason, "evidence %q", evidence)
		assert.Contains(t, receipt.ResolutionReason, "evidence",
			"evidence %q: the reason must name the missing evidence, not just the grades",
			evidence)
	}

	// Same account, same grades, same everything but a held catalog: the only
	// input that moved is the evidence, and it is what decides the status.
	withEvidence := execplane.Evaluate(
		fixtureTierRequest(execplane.ProviderClaude, "ultra", "claude-opus-5",
			execplane.EvidenceProbedCatalog),
		resolution, matched, matched, time.Now(),
	)
	assert.Equal(t, execplane.StatusVerified, withEvidence.VerificationStatus)
}

// TestEvidenceKindDistinguishesEquallyVerifiedProviders covers S4 / REQ-005:
// two providers can both read `verified` while resting on very different
// footing — Codex on a catalog the provider actually served, Claude on nothing
// stronger than "the executor runs the same plan as the reference session".
// The receipt has to keep those apart, because collapsing them into a bare
// status is the silent conflation this gate exists to prevent.
func TestEvidenceKindDistinguishesEquallyVerifiedProviders(t *testing.T) {
	t.Parallel()

	codexGrade := execplane.Entitlement{Grade: "pro", Source: "codex-host@example.test"}
	probed := execplane.Evaluate(
		fixtureTierRequest(execplane.ProviderCodex, "ultra", "gpt-5.6-sol",
			execplane.EvidenceProbedCatalog),
		fixtureDeterminedAccount(execplane.ProviderCodex, "codex-exec@example.test"),
		codexGrade, codexGrade, time.Now(),
	)

	claudeGrade := execplane.Entitlement{Grade: "claude_max", Source: "claude-host@example.test"}
	parity := execplane.Evaluate(
		fixtureTierRequest(execplane.ProviderClaude, "ultra", "claude-opus-5",
			execplane.EvidenceEntitlementParity),
		fixtureDeterminedAccount(execplane.ProviderClaude, "claude-exec@example.test"),
		claudeGrade, claudeGrade, time.Now(),
	)

	require.Equal(t, execplane.StatusVerified, probed.VerificationStatus)
	require.Equal(t, parity.VerificationStatus, probed.VerificationStatus,
		"both providers verify, which is exactly why the status cannot be the whole record")

	assert.Equal(t, execplane.EvidenceProbedCatalog, probed.EvidenceKind)
	assert.Equal(t, execplane.EvidenceEntitlementParity, parity.EvidenceKind)
	assert.NotEqual(t, probed.EvidenceKind, parity.EvidenceKind)
	assert.True(t, probed.Complete())
	assert.True(t, parity.Complete())
}

// TestEvidenceKindSurvivesSerialization covers S4 / REQ-005: the receipt is
// read back later, by someone who was not here. Evidence strength that lives
// only in memory reconstructs nothing, so it travels as a stable JSON key with
// the wire spellings the gate publishes.
func TestEvidenceKindSurvivesSerialization(t *testing.T) {
	t.Parallel()

	// Spelled out rather than compared to the constants alone: renaming a wire
	// value has to break a test rather than move both sides together.
	wire := map[execplane.EvidenceKind]string{
		execplane.EvidenceNone:              "none",
		execplane.EvidenceProbedCatalog:     "probed_catalog",
		execplane.EvidenceEntitlementParity: "entitlement_parity",
	}

	for kind, spelling := range wire {
		receipt := fixtureVerifiedReceipt(t)
		receipt.EvidenceKind = kind

		encoded, err := json.Marshal(receipt)
		require.NoError(t, err, spelling)
		assert.Contains(t, string(encoded), `"evidence_kind":"`+spelling+`"`)

		var decoded execplane.IntegrityReceipt
		require.NoError(t, json.Unmarshal(encoded, &decoded), spelling)
		assert.Equal(t, kind, decoded.EvidenceKind, spelling)
	}
}

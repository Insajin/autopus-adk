package gates

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var decideNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func decideFor(t *testing.T, paths ...string) ApplicabilityReceipt {
	t.Helper()
	receipt := Decide(DecisionInput{
		SpecID:         "SPEC-GATES-001",
		Classification: Classify(paths, nil),
		Now:            decideNow,
	})
	require.Len(t, receipt.Decisions, len(Catalog), "every catalog gate must be decided")
	return receipt
}

func applicabilityOf(t *testing.T, receipt ApplicabilityReceipt, id GateID) GateDecision {
	t.Helper()
	decision, ok := receipt.Decision(id)
	require.True(t, ok, "gate %s missing from receipt", id)
	return decision
}

func TestDecide_SmallUIChange(t *testing.T) {
	receipt := decideFor(t, "frontend/src/components/Button.tsx")
	assert.Equal(t, ClassUIOnly, receipt.ChangeClass)

	ux := applicabilityOf(t, receipt, GateUXVerification)
	assert.Equal(t, Required, ux.Applicability)
	assert.Contains(t, ux.Reason, "UI surface in change set: 1 path(s)")
	assert.Equal(t, Required, applicabilityOf(t, receipt, GateAccessibility).Applicability)

	integration := applicabilityOf(t, receipt, GateIntegration)
	assert.Equal(t, NotApplicable, integration.Applicability)
	assert.Equal(t, "single-domain change set without security or data surface", integration.Reason)

	review := applicabilityOf(t, receipt, GateProviderReview)
	assert.Equal(t, Required, review.Applicability)
	assert.Contains(t, review.Reason, "code change set requires provider review")
}

func TestDecide_SecurityDBChange(t *testing.T) {
	receipt := decideFor(t, "backend/db/migrations/001.sql")
	assert.Equal(t, ClassSecurityOrData, receipt.ChangeClass)

	integration := applicabilityOf(t, receipt, GateIntegration)
	assert.Equal(t, Required, integration.Applicability)
	assert.Contains(t, integration.Reason, "security_or_data change set crosses an integration boundary")

	review := applicabilityOf(t, receipt, GateProviderReview)
	assert.Equal(t, Required, review.Applicability)
	assert.Contains(t, review.Reason, "security_or_data change set requires provider review")

	accessibility := applicabilityOf(t, receipt, GateAccessibility)
	assert.Equal(t, NotApplicable, accessibility.Applicability)
	assert.Equal(t, "no UI surface in change set", accessibility.Reason)
}

func TestDecide_MultiDomainChange(t *testing.T) {
	receipt := decideFor(t, "pkg/a/x.go", "pkg/b/y.go")
	assert.Equal(t, ClassMultiDomain, receipt.ChangeClass)

	integration := applicabilityOf(t, receipt, GateIntegration)
	assert.Equal(t, Required, integration.Applicability)
	assert.Contains(t, integration.Reason, "multi_domain change set crosses an integration boundary")
	assert.Equal(t, NotApplicable, applicabilityOf(t, receipt, GateUXVerification).Applicability)
}

// TestDecide_ScenariosDiffer pins acceptance box (1): the three issue
// scenarios must not collapse into one decision set.
func TestDecide_ScenariosDiffer(t *testing.T) {
	ui := decideFor(t, "frontend/src/components/Button.tsx")
	db := decideFor(t, "backend/db/migrations/001.sql")
	multi := decideFor(t, "pkg/a/x.go", "pkg/b/y.go")

	assert.NotEqual(t, ui.Decisions, db.Decisions)
	assert.NotEqual(t, db.Decisions, multi.Decisions)
	assert.NotEqual(t, ui.Decisions, multi.Decisions)
}

func TestDecide_DocOnlyChange(t *testing.T) {
	receipt := decideFor(t, "README.md", "docs/guide.md")
	assert.Equal(t, ClassDocOnly, receipt.ChangeClass)

	probe := applicabilityOf(t, receipt, GateRiskFirstProbe)
	assert.Equal(t, NotApplicable, probe.Applicability)
	assert.Equal(t, "no integration boundary", probe.Reason)
	for _, id := range []GateID{GateBuild, GateUnitTests, GateAnnotation, GateProviderReview} {
		assert.Equal(t, NotApplicable, applicabilityOf(t, receipt, id).Applicability, "gate %s", id)
	}
	assert.Equal(t, Required, applicabilityOf(t, receipt, GateDocSync).Applicability)
}

func TestDecide_MandatorySafetyGatesNeverNotApplicable(t *testing.T) {
	changeSets := map[ChangeClass][]string{
		ClassDocOnly:        {"README.md"},
		ClassUIOnly:         {"frontend/src/components/Button.tsx"},
		ClassSecurityOrData: {"backend/db/migrations/001.sql"},
		ClassMultiDomain:    {"pkg/a/x.go", "pkg/b/y.go"},
		ClassGeneral:        {"pkg/a/x.go"},
	}
	for class, paths := range changeSets {
		receipt := decideFor(t, paths...)
		require.Equal(t, class, receipt.ChangeClass)
		for _, id := range []GateID{GateSecurity, GateValidation, GateDataLoss, GateDeterministicOracle} {
			decision := applicabilityOf(t, receipt, id)
			assert.True(t, decision.Mandatory, "%s/%s must be flagged mandatory", class, id)
			assert.NotEqual(t, NotApplicable, decision.Applicability, "%s/%s", class, id)
		}
	}
}

func TestDecide_AnnotationBlockedWhenReferenceMissing(t *testing.T) {
	receipt := Decide(DecisionInput{
		SpecID:                     "SPEC-GATES-001",
		Classification:             Classify([]string{"pkg/a/x.go"}, nil),
		AnnotationReferenceMissing: true,
		Now:                        decideNow,
	})
	annotation := applicabilityOf(t, receipt, GateAnnotation)
	assert.Equal(t, Blocked, annotation.Applicability)
	assert.Equal(t, "reference source missing", annotation.Reason)
}

func TestDecide_RequiredGatesNameMissingEvidence(t *testing.T) {
	receipt := decideFor(t, "pkg/a/x.go")
	build := applicabilityOf(t, receipt, GateBuild)
	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonNoPriorEvidence)
	assert.Empty(t, build.InputClosureSHA256)
	assert.Nil(t, build.ReusedEvidence)
	assert.Equal(t, "2026-09-06T12:00:00Z", receipt.GeneratedAt)
	assert.Equal(t, []string{"pkg/a/x.go"}, receipt.ChangedPaths)
}

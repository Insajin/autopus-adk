package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateLaneRowsAppliesBlockingMatrix(t *testing.T) {
	t.Parallel()

	mustGap := LaneRow{Lane: "canary-explicit", LanePolicy: LanePolicyMust, Status: LaneStatusSetupGap, SetupGapClass: SetupGapCanaryTemplate}
	mustGap = NormalizeLaneRow(mustGap)
	assert.Equal(t, SeverityHigh, mustGap.Severity)
	assert.Equal(t, LaneVerdictBlock, mustGap.LaneVerdict)

	optionalFailure := LaneRow{Lane: "gui-explore", LanePolicy: LanePolicyOptional, Status: LaneStatusFailed, SetupGapClass: SetupGapNone, Severity: SeverityMedium}
	optionalFailure = NormalizeLaneRow(optionalFailure)
	assert.Equal(t, LaneVerdictWarn, optionalFailure.LaneVerdict)

	assert.Equal(t, GateStatusBlocked, AggregateGateStatus([]LaneRow{mustGap, optionalFailure}))
}

func TestDeferredLaneWithoutSetupGapIsInformationalPass(t *testing.T) {
	t.Parallel()

	deferred := NormalizeLaneRow(LaneRow{
		Lane:          "mobile-readiness",
		LanePolicy:    LanePolicyDeferred,
		Status:        LaneStatusDeferred,
		SetupGapClass: SetupGapNone,
	})

	assert.Equal(t, SeverityNone, deferred.Severity)
	assert.Equal(t, LaneVerdictPass, deferred.LaneVerdict)
	assert.Equal(t, GateStatusPassed, AggregateGateStatus([]LaneRow{deferred}))
}

func TestInvalidLaneRowCombinationFailsClosed(t *testing.T) {
	t.Parallel()

	row := NormalizeLaneRow(LaneRow{
		Lane:          "fast",
		LanePolicy:    LanePolicyMust,
		Status:        LaneStatusPassed,
		SetupGapClass: SetupGapUnsafeCommand,
		Severity:      SeverityNone,
	})

	assert.Equal(t, LaneStatusBlocked, row.Status)
	assert.Equal(t, SetupGapPolicyForbidden, row.SetupGapClass)
	assert.Equal(t, SeverityHigh, row.Severity)
	assert.Equal(t, LaneVerdictBlock, row.LaneVerdict)
	if assert.Len(t, row.Blockers, 1) {
		assert.Equal(t, "invalid_lane_row_contract", row.Blockers[0].Reason)
	}
}

func TestSetupGapTakesPrecedenceOverWarningNormalization(t *testing.T) {
	t.Parallel()

	row := NormalizeLaneRow(LaneRow{
		Lane:          "canary-explicit",
		LanePolicy:    LanePolicyMust,
		Status:        LaneStatusWarning,
		SetupGapClass: SetupGapCanaryTemplate,
	})

	assert.Equal(t, LaneStatusSetupGap, row.Status)
	assert.Equal(t, SetupGapCanaryTemplate, row.SetupGapClass)
	assert.Equal(t, SeverityHigh, row.Severity)
	assert.Equal(t, LaneVerdictBlock, row.LaneVerdict)
}

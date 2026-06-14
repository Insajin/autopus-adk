package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyMobileOraclePassesOnMatchingExit(t *testing.T) {
	dir := t.TempDir()
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")
	result := commandResult{ExitCode: 0, Status: "passed"}
	check := IndexCheck{ID: "mobile-scripted-smoke", JourneyID: pack.ID, Adapter: pack.Adapter.ID, Expected: "exit_code=0"}

	applyMobileOracle(dir, pack, &result, &check)

	assert.Equal(t, "passed", result.Status)
	assert.Equal(t, "passed", check.Status)
}

func TestApplyMobileOracleFailsOnExitMismatch(t *testing.T) {
	dir := t.TempDir()
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")
	result := commandResult{ExitCode: 1, Status: "passed"}
	check := IndexCheck{ID: "mobile-scripted-smoke", JourneyID: pack.ID, Adapter: pack.Adapter.ID, Expected: "exit_code=0"}

	applyMobileOracle(dir, pack, &result, &check)

	assert.Equal(t, "failed", result.Status)
	assert.Equal(t, "failed", check.Status)
	assert.NotEmpty(t, check.FailureSummary)
}

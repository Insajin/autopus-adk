package experiment_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectFullDepthAudit_RateBoundariesAndStableHash(t *testing.T) {
	t.Parallel()

	taskHash, policyHash := testHash("task-1"), testHash("policy")
	none, err := experiment.SelectFullDepthAudit(taskHash, policyHash, 0)
	require.NoError(t, err)
	assert.False(t, none.Selected)

	all, err := experiment.SelectFullDepthAudit(taskHash, policyHash, 100)
	require.NoError(t, err)
	assert.True(t, all.Selected)

	left, err := experiment.SelectFullDepthAudit(taskHash, policyHash, 10)
	require.NoError(t, err)
	right, err := experiment.SelectFullDepthAudit(taskHash, policyHash, 10)
	require.NoError(t, err)
	assert.Equal(t, left, right)
	assert.Equal(t, "sha256_mod_100_v1", left.Algorithm)
}

func TestSelectFullDepthAudit_RejectsMutableIdentity(t *testing.T) {
	_, err := experiment.SelectFullDepthAudit("task-1", "policy-hash", 10)
	require.Error(t, err)
}

func TestBuildRolloutReceipt_DefaultsToShadow(t *testing.T) {
	t.Parallel()

	receipt := experiment.BuildRolloutReceipt(validRolloutInput(), rolloutNow())

	assert.Equal(t, "shadow", receipt.ReceiptKind)
	assert.Equal(t, "SHADOW", receipt.Decision)
	assert.Equal(t, "full_ultra", receipt.ActiveProfile)
}

func TestBuildRolloutReceipt_AuditSampleIsFullDepth(t *testing.T) {
	t.Parallel()

	in := validRolloutInput()
	in.ReceiptKind = "audit_sample"
	in.RiskTier = "low"
	in.FullDepth = true
	in.AuditSelection = experiment.AuditSelection{
		Selected: true, Bucket: 7, RatePercent: 10, Algorithm: "sha256_mod_100_v1",
	}
	receipt := experiment.BuildRolloutReceipt(in, rolloutNow())

	assert.Equal(t, "audit_sample", receipt.ReceiptKind)
	assert.True(t, receipt.FullDepth)
	assert.Equal(t, "audit_sample", receipt.SelectionReason)
	assert.NotEqual(t, "ROLLBACK", receipt.Decision)
}

func TestBuildRolloutReceipt_EligibleCanary_UsesRiskBoundProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		riskTier      string
		wantProfile   string
		wantFullDepth bool
		wantSelection string
	}{
		{name: "high stays full depth", riskTier: "high", wantProfile: "full_ultra", wantFullDepth: true, wantSelection: "risk_requires_full_depth"},
		{name: "critical stays full depth", riskTier: "critical", wantProfile: "full_ultra", wantFullDepth: true, wantSelection: "risk_requires_full_depth"},
		{name: "sensitive stays full depth", riskTier: "sensitive", wantProfile: "full_ultra", wantFullDepth: true, wantSelection: "risk_requires_full_depth"},
		{name: "unknown stays full depth", riskTier: "unknown", wantProfile: "full_ultra", wantFullDepth: true, wantSelection: "risk_requires_full_depth"},
		{name: "low remains compact", riskTier: "low", wantProfile: "compact_ultra", wantFullDepth: false, wantSelection: "eligible_canary"},
		{name: "medium remains compact", riskTier: "medium", wantProfile: "compact_ultra", wantFullDepth: false, wantSelection: "eligible_canary"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := validRolloutInput()
			in.ReceiptKind = "canary"
			in.RiskTier = tt.riskTier
			in.FullDepth = true
			in.Promotion.RolloutDecision = "ELIGIBLE_NEXT_CANARY"

			receipt := experiment.BuildRolloutReceipt(in, rolloutNow())

			assert.Equal(t, "canary", receipt.ReceiptKind)
			assert.Equal(t, "CANARY", receipt.Decision)
			assert.Equal(t, tt.wantProfile, receipt.ActiveProfile)
			assert.Equal(t, tt.wantFullDepth, receipt.FullDepth)
			assert.Equal(t, tt.wantSelection, receipt.SelectionReason)
		})
	}
}

func TestBuildRolloutReceipt_SensitiveCompactFailsPolicyAndRollsBack(t *testing.T) {
	t.Parallel()

	in := validRolloutInput()
	in.ReceiptKind = "canary"
	in.RiskTier = "critical"
	in.Sensitive = true
	in.FullDepth = false
	receipt := experiment.BuildRolloutReceipt(in, rolloutNow())

	assert.Equal(t, "rollback", receipt.ReceiptKind)
	assert.Equal(t, "ROLLBACK", receipt.Decision)
	assert.Equal(t, "full_ultra", receipt.ActiveProfile)
	assert.Contains(t, receipt.ReasonCodes, "sensitive_compact_policy")
}

func TestBuildRolloutReceipt_UnknownAndMalformedRiskCannotCanaryCompact(t *testing.T) {
	for _, risk := range []string{"", "malformed", "unknown"} {
		in := validRolloutInput()
		in.ReceiptKind = "canary"
		in.RiskTier = risk
		in.FullDepth = false
		in.Promotion.RolloutDecision = "ELIGIBLE_NEXT_CANARY"
		receipt := experiment.BuildRolloutReceipt(in, rolloutNow())
		assert.Equal(t, "rollback", receipt.ReceiptKind)
		assert.Equal(t, "full_ultra", receipt.ActiveProfile)
	}
}

func TestBuildRolloutReceipt_RollbackPrecedesAudit(t *testing.T) {
	in := validRolloutInput()
	in.ReceiptKind = "audit_sample"
	in.AuditSelection.Selected = true
	in.Promotion = experiment.PromotionResult{RolloutDecision: "ROLLBACK", ReasonCodes: []string{"quality_failed"}}
	receipt := experiment.BuildRolloutReceipt(in, rolloutNow())
	assert.Equal(t, "rollback", receipt.ReceiptKind)
	assert.Contains(t, receipt.ReasonCodes, "quality_failed")
}

func TestBuildRolloutReceipt_ContainsNoRawPromptResponseOrAbsolutePath(t *testing.T) {
	t.Parallel()

	receipt := experiment.BuildRolloutReceipt(validRolloutInput(), rolloutNow())
	raw, err := json.Marshal(receipt)
	require.NoError(t, err)
	text := string(raw)

	for _, forbidden := range []string{"prompt", "response", "/Users/", `C:\\Users\\`} {
		assert.NotContains(t, strings.ToLower(text), strings.ToLower(forbidden))
	}
	for _, secret := range []string{"task-corpus-hash", "policy-hash", "config-hash", "ghp_SECRET", "a.b.c"} {
		assert.NotContains(t, text, secret)
	}
	assert.Contains(t, text, "sha256:")
}

func TestBuildRolloutReceipt_ExperimentIDIsAlwaysDigested(t *testing.T) {
	for _, value := range []string{"exp-1", "sk-secret-value", "eyJhbGciOiJIUzI1NiJ9.payload.signature"} {
		in := validRolloutInput()
		in.ExperimentID = value
		receipt := experiment.BuildRolloutReceipt(in, rolloutNow())
		assert.True(t, strings.HasPrefix(receipt.ExperimentID, "sha256:"))
		assert.NotContains(t, receipt.ExperimentID, value)
	}
}

func testHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validRolloutInput() experiment.RolloutReceiptInput {
	return experiment.RolloutReceiptInput{
		ExperimentID: "exp-1", TaskCorpusHash: "task-corpus-hash",
		PolicyHash: "policy-hash", ConfigHash: "config-hash",
		RiskTier: "low", FullDepth: true,
		Promotion: experiment.PromotionResult{
			RolloutDecision: "BLOCKED", Provisional25PctTarget: "NOT_MET",
		},
	}
}

func rolloutNow() time.Time {
	return time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
}

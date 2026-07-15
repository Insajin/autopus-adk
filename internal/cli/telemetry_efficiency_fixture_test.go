package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func efficiencyEvidenceFixture(t *testing.T) telemetryEfficiencyEvidence {
	t.Helper()
	identity := efficiencyIdentityFixture()
	calls := make([]experiment.CallEvidence, 0, 20)
	for index := 0; index < 20; index++ {
		usage := efficiencyActualUsage(identity, "measure", fmt.Sprintf("call-%02d", index), "", 100)
		if index == 19 {
			usage = telemetry.NormalizeUsage(telemetry.UsageInput{
				RunID: "measure", CallID: "call-19", Source: telemetry.UsageSourceProvider,
				Provider: identity.Provider, ProviderVersion: identity.ProviderVersion,
				Model: identity.Model, ModelVersion: identity.ModelVersion, Effort: identity.EffortPolicy,
				RiskPolicy: identity.RiskPolicy, CacheStratum: identity.CacheStratum, ConfigHash: identity.ConfigHash,
			})
		}
		calls = append(calls, experiment.CallEvidence{Usage: usage, Identity: identity})
	}
	trials := []experiment.TaskTrial{
		efficiencyTrial(identity, "task-a", "baseline", "AB", 1000),
		efficiencyTrial(identity, "task-a", "candidate", "AB", 700),
		efficiencyTrial(identity, "task-b", "baseline", "BA", 2000),
		efficiencyTrial(identity, "task-b", "candidate", "BA", 1400),
	}
	return telemetryEfficiencyEvidence{
		Version:         telemetryEfficiencyEvidenceVersion,
		Calls:           calls,
		ExpectedTaskIDs: []string{"task-a", "task-b"},
		QualityOutcomes: []experiment.QualityOutcomeEvidence{
			efficiencyQualityOutcome("task-a", "low"),
			efficiencyQualityOutcome("task-b", "critical"),
		},
		Neutrality: experiment.NeutralityEvidence{
			BaselineObjectiveHash: "objective", CandidateObjectiveHash: "objective",
			BaselineCallPolicyHash: "calls", CandidateCallPolicyHash: "calls",
			BaselineAcceptanceHash: "acceptance", CandidateAcceptanceHash: "acceptance",
		},
		Trials: trials, PolicyParityPassed: true, ContextIntegrityPassed: true,
		Reliability:  &telemetryEfficiencyReliabilityEvidence{Blocked: false, ExitCode: 0, Reason: "ok", AttributedVersion: "candidate"},
		CurrentStage: "shadow", CandidateBehaviorActive: false,
		Rollout: telemetryEfficiencyRolloutEvidence{
			ExperimentID: "/private/customer/raw-experiment-id", TaskCorpusHash: efficiencyHash("corpus"),
			PolicyHash: efficiencyHash("policy"), ConfigHash: efficiencyHash("config"), ReceiptKind: "canary",
			RiskTier: "low", FullDepth: true,
			AuditTaskHash: efficiencyHash("task-task-a"), AuditRatePercent: 0,
		},
	}
}

func efficiencyQualityOutcome(taskID, risk string) experiment.QualityOutcomeEvidence {
	expected := efficiencyHash("oracle-" + taskID)
	return experiment.QualityOutcomeEvidence{
		TaskID: taskID, TaskHash: efficiencyHash("task-" + taskID), RiskTier: risk,
		ExpectedOracleHash: expected, BaselineObservedOracleHash: expected, CandidateObservedOracleHash: expected,
		BaselineVerificationExitCode: 0, CandidateVerificationExitCode: 0,
		BaselineSecurityStatus: telemetry.StatusPass, CandidateSecurityStatus: telemetry.StatusPass,
		BaselineSecurityReceiptHash:  efficiencyHash("baseline-security-" + taskID),
		CandidateSecurityReceiptHash: efficiencyHash("candidate-security-" + taskID),
	}
}

func efficiencyHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%x", digest[:])
}

func efficiencyIdentityFixture() experiment.ComparisonIdentity {
	return experiment.ComparisonIdentity{
		Provider: "claude", ProviderVersion: "2.1.154", Model: "claude-opus-4-8",
		ModelVersion: "2026-07-01", EffortPolicy: "max", RiskPolicy: "risk-v1",
		CacheStratum: "cold", ConfigHash: "config-hash",
	}
}

func efficiencyTrial(identity experiment.ComparisonIdentity, taskID, arm, order string, raw int64) experiment.TaskTrial {
	return experiment.TaskTrial{
		TaskID: taskID, Arm: arm, PairOrder: order, Identity: identity,
		Runs: []telemetry.AgentRun{{
			AgentName: "executor", TaskID: taskID, Attempt: 1, Status: telemetry.StatusPass,
			AcceptanceStatus: telemetry.StatusPass,
			Usage:            []telemetry.UsageEnvelope{efficiencyActualUsage(identity, "run-"+taskID+"-"+arm, "call-"+taskID+"-"+arm, taskID, raw)},
		}},
	}
}

func efficiencyActualUsage(identity experiment.ComparisonIdentity, runID, callID, taskID string, raw int64) telemetry.UsageEnvelope {
	zero := int64(0)
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: runID, CallID: callID, TaskID: taskID, Source: telemetry.UsageSourceProvider,
		Provider: identity.Provider, ProviderVersion: identity.ProviderVersion,
		Model: identity.Model, ModelVersion: identity.ModelVersion, Effort: identity.EffortPolicy,
		RiskPolicy: identity.RiskPolicy, CacheStratum: identity.CacheStratum, ConfigHash: identity.ConfigHash,
		InputTokensTotal: &raw, OutputTokensTotal: &zero,
	})
}

func writeEfficiencyEvidence(t *testing.T, evidence telemetryEfficiencyEvidence) string {
	t.Helper()
	data, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

package experiment

import (
	"errors"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

const maxQualityVerificationExitCode = 255

// QualityOutcomeEvidence binds deterministic objective and security results to
// one expected paired task without retaining raw logs or provider output.
type QualityOutcomeEvidence struct {
	TaskID                        string `json:"task_id"`
	TaskHash                      string `json:"task_hash"`
	RiskTier                      string `json:"risk_tier"`
	ExpectedOracleHash            string `json:"expected_oracle_hash"`
	BaselineObservedOracleHash    string `json:"baseline_observed_oracle_hash"`
	CandidateObservedOracleHash   string `json:"candidate_observed_oracle_hash"`
	BaselineVerificationExitCode  int    `json:"baseline_verification_exit_code"`
	CandidateVerificationExitCode int    `json:"candidate_verification_exit_code"`
	BaselineSecurityStatus        string `json:"baseline_security_status"`
	CandidateSecurityStatus       string `json:"candidate_security_status"`
	BaselineSecurityReceiptHash   string `json:"baseline_security_receipt_hash"`
	CandidateSecurityReceiptHash  string `json:"candidate_security_receipt_hash"`
}

// QualityResult is the derived completeness and regression ledger. Task IDs
// remain internal until the CLI digests them for output.
type QualityResult struct {
	ExpectedTaskCount       int                  `json:"expected_task_count"`
	OutcomeRowCount         int                  `json:"outcome_row_count"`
	Complete                bool                 `json:"complete"`
	Consistent              bool                 `json:"consistent"`
	ObjectivePassCount      int                  `json:"objective_pass_count"`
	SecurityPassCount       int                  `json:"security_pass_count"`
	MissingTaskIDs          []string             `json:"missing_task_ids"`
	DuplicateTaskIDs        []string             `json:"duplicate_task_ids"`
	UnexpectedTaskIDs       []string             `json:"unexpected_task_ids"`
	InconsistentTaskIDs     []string             `json:"inconsistent_task_ids"`
	CandidateFailureTaskIDs []string             `json:"candidate_failure_task_ids"`
	DerivedRegressions      []RegressionEvidence `json:"derived_regressions"`
}

// ValidateQualityOutcome rejects fields that cannot be used as deterministic,
// sanitized evidence.
func ValidateQualityOutcome(outcome QualityOutcomeEvidence) error {
	if !validQualityTaskID(outcome.TaskID) || !validQualityRisk(outcome.RiskTier) {
		return errors.New("invalid quality identity")
	}
	for _, value := range []string{
		outcome.TaskHash, outcome.ExpectedOracleHash,
		outcome.BaselineObservedOracleHash, outcome.CandidateObservedOracleHash,
		outcome.BaselineSecurityReceiptHash, outcome.CandidateSecurityReceiptHash,
	} {
		if !validCanonicalQualityHash(value) {
			return errors.New("invalid quality hash")
		}
	}
	if outcome.BaselineVerificationExitCode < 0 || outcome.BaselineVerificationExitCode > maxQualityVerificationExitCode ||
		outcome.CandidateVerificationExitCode < 0 || outcome.CandidateVerificationExitCode > maxQualityVerificationExitCode {
		return errors.New("invalid quality verification exit code")
	}
	if !validQualityStatus(outcome.BaselineSecurityStatus) || !validQualityStatus(outcome.CandidateSecurityStatus) {
		return errors.New("invalid quality security status")
	}
	return nil
}

// EvaluateQualityOutcomes derives objective results and regressions from
// observed hashes and deterministic gate results. Caller-supplied regression
// counts are not inputs to this calculation.
func EvaluateQualityOutcomes(expectedTaskIDs []string, outcomes []QualityOutcomeEvidence) QualityResult {
	result := QualityResult{ExpectedTaskCount: len(expectedTaskIDs), OutcomeRowCount: len(outcomes)}
	expected := make(map[string]struct{}, len(expectedTaskIDs))
	for _, taskID := range expectedTaskIDs {
		expected[taskID] = struct{}{}
	}
	rows := make(map[string][]QualityOutcomeEvidence, len(outcomes))
	unexpected := make(map[string]struct{})
	for _, outcome := range outcomes {
		rows[outcome.TaskID] = append(rows[outcome.TaskID], outcome)
		if _, found := expected[outcome.TaskID]; !found {
			unexpected[outcome.TaskID] = struct{}{}
		}
	}
	for _, taskID := range expectedTaskIDs {
		switch len(rows[taskID]) {
		case 0:
			result.MissingTaskIDs = append(result.MissingTaskIDs, taskID)
		case 1:
			evaluateQualityRow(&result, rows[taskID][0])
		default:
			result.DuplicateTaskIDs = append(result.DuplicateTaskIDs, taskID)
		}
	}
	for taskID := range unexpected {
		result.UnexpectedTaskIDs = append(result.UnexpectedTaskIDs, taskID)
	}
	sort.Strings(result.MissingTaskIDs)
	sort.Strings(result.DuplicateTaskIDs)
	sort.Strings(result.UnexpectedTaskIDs)
	sort.Strings(result.InconsistentTaskIDs)
	sort.Strings(result.CandidateFailureTaskIDs)
	result.Complete = len(expectedTaskIDs) > 0 && len(outcomes) == len(expectedTaskIDs) &&
		len(result.MissingTaskIDs) == 0 && len(result.DuplicateTaskIDs) == 0 && len(result.UnexpectedTaskIDs) == 0
	result.Consistent = result.Complete && len(result.InconsistentTaskIDs) == 0
	return result
}

func evaluateQualityRow(result *QualityResult, outcome QualityOutcomeEvidence) {
	if ValidateQualityOutcome(outcome) != nil {
		result.InconsistentTaskIDs = append(result.InconsistentTaskIDs, outcome.TaskID)
		return
	}
	baselineObjective := outcome.BaselineVerificationExitCode == 0 &&
		outcome.BaselineObservedOracleHash == outcome.ExpectedOracleHash
	candidateObjective := outcome.CandidateVerificationExitCode == 0 &&
		outcome.CandidateObservedOracleHash == outcome.ExpectedOracleHash
	baselineSecurity := outcome.BaselineSecurityStatus == telemetry.StatusPass
	candidateSecurity := outcome.CandidateSecurityStatus == telemetry.StatusPass
	if candidateObjective {
		result.ObjectivePassCount++
	}
	if candidateSecurity {
		result.SecurityPassCount++
	}
	if !baselineObjective || !baselineSecurity {
		result.InconsistentTaskIDs = append(result.InconsistentTaskIDs, outcome.TaskID)
		return
	}
	if !candidateObjective || !candidateSecurity {
		result.CandidateFailureTaskIDs = append(result.CandidateFailureTaskIDs, outcome.TaskID)
	}
	if !highCriticalQualityRisk(outcome.RiskTier) {
		return
	}
	if !candidateObjective {
		result.DerivedRegressions = append(result.DerivedRegressions, qualityRegression(outcome, "objective"))
	}
	if !candidateSecurity {
		result.DerivedRegressions = append(result.DerivedRegressions, qualityRegression(outcome, "security"))
	}
}

func qualityRegression(outcome QualityOutcomeEvidence, kind string) RegressionEvidence {
	return RegressionEvidence{TaskID: outcome.TaskID, Kind: kind, Risk: outcome.RiskTier,
		BaselineOutcome: telemetry.StatusPass, CandidateOutcome: telemetry.StatusFail}
}

func validQualityTaskID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-_.:", char)) {
			return false
		}
	}
	return true
}

func validCanonicalQualityHash(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[len("sha256:"):] {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

func validQualityRisk(value string) bool {
	return value == "low" || value == "medium" || value == "high" || value == "critical"
}

func highCriticalQualityRisk(value string) bool {
	return value == "high" || value == "critical"
}

func validQualityStatus(value string) bool {
	return value == telemetry.StatusPass || value == telemetry.StatusFail
}

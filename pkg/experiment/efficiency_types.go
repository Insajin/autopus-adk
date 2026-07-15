package experiment

import (
	"time"

	"github.com/insajin/autopus-adk/pkg/evalregression"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

// CodexReviewEffortPolicyV1 binds review roles to their actual Codex efforts.
const CodexReviewEffortPolicyV1 = "codex_review_xhigh_security_max_v1"

// ComparisonIdentity is the complete compatibility stratum for paired calls.
type ComparisonIdentity struct {
	Provider        string `json:"provider"`
	ProviderVersion string `json:"provider_version"`
	Model           string `json:"model"`
	ModelVersion    string `json:"model_version"`
	EffortPolicy    string `json:"effort_policy"`
	RiskPolicy      string `json:"risk_policy"`
	CacheStratum    string `json:"cache_stratum"`
	ConfigHash      string `json:"config_hash"`
}

type CallEvidence struct {
	Usage    telemetry.UsageEnvelope `json:"usage"`
	Identity ComparisonIdentity      `json:"identity"`
}

type NeutralityEvidence struct {
	BaselineObjectiveHash   string `json:"baseline_objective_hash"`
	CandidateObjectiveHash  string `json:"candidate_objective_hash"`
	BaselineCallPolicyHash  string `json:"baseline_call_policy_hash"`
	CandidateCallPolicyHash string `json:"candidate_call_policy_hash"`
	BaselineAcceptanceHash  string `json:"baseline_acceptance_hash"`
	CandidateAcceptanceHash string `json:"candidate_acceptance_hash"`
}

type MeasurementResult struct {
	ActualUsageCapturePct float64  `json:"actual_usage_capture_pct"`
	MeasurementGate       string   `json:"measurement_gate"`
	NeutralityGate        string   `json:"neutrality_gate"`
	RolloutDecision       string   `json:"rollout_decision"`
	ReasonCodes           []string `json:"reason_codes,omitempty"`
}

type TaskTrial struct {
	TaskID    string               `json:"task_id"`
	Arm       string               `json:"arm"`
	PairOrder string               `json:"pair_order"`
	Identity  ComparisonIdentity   `json:"identity"`
	Runs      []telemetry.AgentRun `json:"runs"`
}

type ExcludedTask struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

type PairedComparison struct {
	ExpectedTaskIDs             []string       `json:"expected_task_ids"`
	UnexpectedTaskIDs           []string       `json:"unexpected_task_ids"`
	ExpectedTaskCount           int            `json:"expected_task_count"`
	PairedExpectedTaskCount     int            `json:"paired_expected_task_count"`
	ExpectedCorpusComplete      bool           `json:"expected_corpus_complete"`
	PairedTaskIDs               []string       `json:"paired_task_ids"`
	UnpairedTaskIDs             []string       `json:"unpaired_task_ids"`
	ExcludedTasks               []ExcludedTask `json:"excluded_tasks"`
	ABTaskIDs                   []string       `json:"ab_task_ids"`
	BATaskIDs                   []string       `json:"ba_task_ids"`
	PairedARawTokens            int64          `json:"paired_a_raw_tokens"`
	PairedBRawTokens            int64          `json:"paired_b_raw_tokens"`
	PairedReductionPct          float64        `json:"paired_reduction_pct"`
	MedianPairedRawReductionPct float64        `json:"median_paired_raw_reduction_pct"`
	Provisional25PctTarget      string         `json:"provisional_25_pct_target"`
	PairedTaskCount             int            `json:"paired_task_count"`
}

type PromotionInput struct {
	Measurement             MeasurementResult
	Comparison              PairedComparison
	Quality                 *QualityResult
	Regressions             []RegressionEvidence
	UsageConflict           bool
	PolicyParityPassed      bool
	ContextIntegrityPassed  bool
	ReliabilityDecision     *evalregression.GateDecision
	CurrentStage            string
	CandidateBehaviorActive bool
}

// RegressionEvidence is objective regression input, not a precomputed count.
type RegressionEvidence struct {
	TaskID           string `json:"task_id"`
	Kind             string `json:"kind"` // objective or security
	Risk             string `json:"risk"`
	BaselineOutcome  string `json:"baseline_outcome"`
	CandidateOutcome string `json:"candidate_outcome"`
}

type PromotionResult struct {
	HighCriticalRegressions       int      `json:"high_critical_regressions"`
	MeasuredMedianRawReductionPct float64  `json:"measured_median_raw_reduction_pct"`
	Provisional25PctTarget        string   `json:"provisional_25_pct_target"`
	RolloutDecision               string   `json:"rollout_decision"`
	ReasonCodes                   []string `json:"reason_codes,omitempty"`
}

type AuditSelection struct {
	Selected    bool   `json:"selected"`
	Bucket      int    `json:"bucket"`
	RatePercent int    `json:"rate_percent"`
	Algorithm   string `json:"algorithm"`
}

type RolloutReceiptInput struct {
	ExperimentID   string
	TaskCorpusHash string
	PolicyHash     string
	ConfigHash     string
	ReceiptKind    string
	RiskTier       string
	Sensitive      bool
	FullDepth      bool
	AuditSelection AuditSelection
	Promotion      PromotionResult
}

type RolloutReceipt struct {
	Version         int            `json:"version"`
	ExperimentID    string         `json:"experiment_id"`
	TaskCorpusHash  string         `json:"task_corpus_hash"`
	PolicyHash      string         `json:"policy_hash"`
	ConfigHash      string         `json:"config_hash"`
	RecordedAt      time.Time      `json:"recorded_at"`
	ReceiptKind     string         `json:"receipt_kind"`
	Decision        string         `json:"decision"`
	ActiveProfile   string         `json:"active_profile"`
	RiskTier        string         `json:"risk_tier"`
	FullDepth       bool           `json:"full_depth"`
	SelectionReason string         `json:"selection_reason,omitempty"`
	AuditSelection  AuditSelection `json:"audit_selection"`
	ReasonCodes     []string       `json:"reason_codes,omitempty"`
}

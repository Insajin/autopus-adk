package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/spf13/cobra"
)

const (
	telemetryEfficiencyEvidenceVersion  = 1
	telemetryEfficiencyResultVersion    = 1
	maxTelemetryEfficiencyEvidenceBytes = 1 << 20
	maxTelemetryEfficiencyCalls         = 10000
	maxTelemetryEfficiencyTrials        = 2000
	maxTelemetryEfficiencyExpectedTasks = maxTelemetryEfficiencyTrials / 2
	maxTelemetryEfficiencyRunsPerTrial  = 1000
	maxTelemetryEfficiencyRegressions   = 10000
	maxTelemetryEfficiencyStringBytes   = 4096
)

type telemetryEfficiencyReliabilityEvidence struct {
	Blocked           bool   `json:"blocked"`
	ExitCode          int    `json:"exit_code"`
	Reason            string `json:"reason"`
	AttributedVersion string `json:"attributed_version"`
}

type telemetryEfficiencyRolloutEvidence struct {
	ExperimentID     string `json:"experiment_id"`
	TaskCorpusHash   string `json:"task_corpus_hash"`
	PolicyHash       string `json:"policy_hash"`
	ConfigHash       string `json:"config_hash"`
	ReceiptKind      string `json:"receipt_kind"`
	RiskTier         string `json:"risk_tier"`
	Sensitive        bool   `json:"sensitive"`
	FullDepth        bool   `json:"full_depth"`
	AuditTaskHash    string `json:"audit_task_hash,omitempty"`
	AuditRatePercent int    `json:"audit_rate_percent,omitempty"`
}

type telemetryEfficiencyEvidence struct {
	Version                 int                                     `json:"version"`
	Calls                   []experiment.CallEvidence               `json:"calls"`
	Neutrality              experiment.NeutralityEvidence           `json:"neutrality"`
	ExpectedTaskIDs         []string                                `json:"expected_task_ids"`
	Trials                  []experiment.TaskTrial                  `json:"trials"`
	QualityOutcomes         []experiment.QualityOutcomeEvidence     `json:"quality_outcomes"`
	Regressions             []experiment.RegressionEvidence         `json:"regressions"`
	UsageConflict           bool                                    `json:"usage_conflict"`
	PolicyParityPassed      bool                                    `json:"policy_parity_passed"`
	ContextIntegrityPassed  bool                                    `json:"context_integrity_passed"`
	Reliability             *telemetryEfficiencyReliabilityEvidence `json:"reliability"`
	CurrentStage            string                                  `json:"current_stage"`
	CandidateBehaviorActive bool                                    `json:"candidate_behavior_active"`
	Rollout                 telemetryEfficiencyRolloutEvidence      `json:"rollout"`
}

type telemetryEfficiencyResult struct {
	Version        int                          `json:"version"`
	Measurement    experiment.MeasurementResult `json:"measurement"`
	Comparison     experiment.PairedComparison  `json:"comparison"`
	Quality        experiment.QualityResult     `json:"quality"`
	Promotion      experiment.PromotionResult   `json:"promotion"`
	RolloutReceipt experiment.RolloutReceipt    `json:"rollout_receipt"`
}

func newTelemetryEfficiencyCmd() *cobra.Command {
	var evidencePath, format string
	cmd := &cobra.Command{
		Use:           "efficiency",
		Short:         "Evaluate Ultra efficiency evidence and rollout eligibility",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return errors.New("telemetry efficiency: positional arguments are not supported")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if evidencePath == "" {
				return errors.New("telemetry efficiency: --evidence-json is required")
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if format != "json" && format != "human" {
				return errors.New("telemetry efficiency: unsupported format")
			}
			evidence, err := readTelemetryEfficiencyEvidence(evidencePath)
			if err != nil {
				return err
			}
			result := evaluateTelemetryEfficiency(evidence, time.Now())
			if format == "human" {
				return writeTelemetryEfficiencyHuman(cmd, result)
			}
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
				return errors.New("telemetry efficiency: write failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&evidencePath, "evidence-json", "", "Strict JSON efficiency evidence file")
	cmd.Flags().StringVar(&format, "format", "human", "Output format (human|json)")
	return cmd
}

func readTelemetryEfficiencyEvidence(path string) (telemetryEfficiencyEvidence, error) {
	var evidence telemetryEfficiencyEvidence
	file, err := os.Open(path)
	if err != nil {
		return evidence, errors.New("telemetry efficiency: evidence unavailable")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxTelemetryEfficiencyEvidenceBytes+1))
	if err != nil {
		return evidence, errors.New("telemetry efficiency: evidence unavailable")
	}
	if len(data) > maxTelemetryEfficiencyEvidenceBytes {
		return evidence, errors.New("telemetry efficiency: evidence exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&evidence) != nil || ensureTelemetryEfficiencyEOF(decoder) != nil || validateTelemetryEfficiencyEvidence(evidence) != nil {
		return telemetryEfficiencyEvidence{}, errors.New("telemetry efficiency: invalid evidence")
	}
	return evidence, nil
}

func ensureTelemetryEfficiencyEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}

func writeTelemetryEfficiencyHuman(cmd *cobra.Command, result telemetryEfficiencyResult) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(),
		"measurement=%s neutrality=%s paired_tasks=%d quality_complete=%t median_reduction=%.3f%% promotion=%s receipt=%s profile=%s\n",
		result.Measurement.MeasurementGate, result.Measurement.NeutralityGate, result.Comparison.PairedTaskCount,
		result.Quality.Complete, result.Comparison.MedianPairedRawReductionPct, result.Promotion.RolloutDecision,
		result.RolloutReceipt.Decision, result.RolloutReceipt.ActiveProfile)
	if err != nil {
		return errors.New("telemetry efficiency: write failed")
	}
	return nil
}

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

const (
	pipelineOMPContextCohortSchema    = "autopus.omp_context_cohort.v1"
	pipelineOMPContextCohortTaskCount = 20
	pipelineOMPContextCohortMaxPrompt = 256 << 10
	pipelineOMPContextCohortMaxBodies = 4 << 20
)

var (
	pipelineOMPContextCohortTaskIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	pipelineOMPContextCohortLocatorPattern = regexp.MustCompile(`^AUTOPUS_OMP_CONTEXT_PROVIDER_[A-Za-z0-9_]+$`)
)

type pipelineOMPContextCohortTask struct {
	TaskID string
	Prompt string
}

type pipelineOMPContextCohortCall struct {
	TaskID            string
	Prompt            string
	CredentialLocator string
	Variant           promptlayer.OMPContextCanaryVariantV1
	Order             int
}

type pipelineOMPContextCohortObservation struct {
	Tokens           int64
	IntegrityPassed  bool
	SecurityPassed   bool
	QualityScore     int64
	FallbackVerified bool
	RollbackVerified bool
	OutputBody       string
}

type pipelineOMPContextCohortRunner func(
	context.Context,
	pipelineOMPContextCohortCall,
) (pipelineOMPContextCohortObservation, error)

type pipelineOMPContextCohortInput struct {
	LiveOptIn         string
	CredentialLocator string
	Tasks             []pipelineOMPContextCohortTask
	Run               pipelineOMPContextCohortRunner
	Log               io.Writer
}

type pipelineOMPContextCohortReceipt struct {
	SchemaVersion              string                                  `json:"schema_version"`
	EvidenceClass              string                                  `json:"evidence_class"`
	Status                     string                                  `json:"status"`
	Reason                     string                                  `json:"reason"`
	LiveOptIn                  string                                  `json:"live_opt_in"`
	CredentialLocator          string                                  `json:"credential_locator"`
	TaskCount                  int                                     `json:"task_count"`
	PrimaryCallCount           int                                     `json:"primary_call_count"`
	TaskDigests                []string                                `json:"task_digests"`
	PromptDigests              []string                                `json:"prompt_digests"`
	PairCount                  int                                     `json:"pair_count"`
	ABCount                    int                                     `json:"ab_count"`
	BACount                    int                                     `json:"ba_count"`
	MedianReductionBasisPoints int64                                   `json:"median_reduction_basis_points"`
	Gates                      []promptlayer.OMPContextPromotionGateV1 `json:"gates"`
}

type pipelineOMPContextCohortResult struct {
	Rows    []promptlayer.OMPContextCanaryRowV1
	Receipt pipelineOMPContextCohortReceipt
}

// @AX:WARN [AUTO]: Fail-closed shadow-candidate cohort flow has eight conditional rejection branches.
// @AX:REASON: Preserve every rejection path; this receipt is evaluation evidence and grants no production authority.
func runPipelineOMPContextCohort(
	ctx context.Context,
	input pipelineOMPContextCohortInput,
) (pipelineOMPContextCohortResult, error) {
	result := pipelineOMPContextCohortResult{
		Rows: []promptlayer.OMPContextCanaryRowV1{},
		Receipt: pipelineOMPContextCohortReceipt{
			SchemaVersion: pipelineOMPContextCohortSchema,
			EvidenceClass: "shadow-candidate",
			Status:        "rejected", Reason: "input-invalid",
			TaskCount: len(input.Tasks), TaskDigests: []string{}, PromptDigests: []string{},
			Gates: []promptlayer.OMPContextPromotionGateV1{},
		},
	}
	if reason := validatePipelineOMPContextCohortInput(input, &result.Receipt); reason != "" {
		result.Receipt.Reason = reason
		return result, fmt.Errorf("OMP context cohort rejected: reason=%s", reason)
	}

	for taskIndex, task := range input.Tasks {
		variants := pipelineOMPContextCohortOrder(taskIndex)
		for order, variant := range variants {
			callIndex := len(result.Rows) + 1
			if ctx == nil || ctx.Err() != nil {
				result.Receipt.Reason = "context-unavailable"
				return result, pipelineOMPContextCohortCallError(callIndex, task.TaskID, result.Receipt.Reason)
			}
			observation, err := input.Run(ctx, pipelineOMPContextCohortCall{
				TaskID: task.TaskID, Prompt: task.Prompt, CredentialLocator: input.CredentialLocator,
				Variant: variant, Order: order + 1,
			})
			result.Receipt.PrimaryCallCount++
			if err != nil {
				result.Receipt.Reason = "provider-call-failed"
				return result, pipelineOMPContextCohortCallError(callIndex, task.TaskID, result.Receipt.Reason)
			}
			if observation.Tokens <= 0 {
				result.Receipt.Reason = "invalid-observation"
				return result, pipelineOMPContextCohortCallError(callIndex, task.TaskID, result.Receipt.Reason)
			}
			result.Rows = append(result.Rows, pipelineOMPContextCohortRow(task.TaskID, variant, order+1, observation))
			observation.OutputBody = ""
		}
	}

	aggregate, err := promptlayer.ReduceOMPContextCanaryPairsV1(result.Rows)
	if err != nil {
		result.Receipt.Reason = "canary-reduction-failed"
		return result, fmt.Errorf("OMP context cohort rejected: reason=%s", result.Receipt.Reason)
	}
	decision := promptlayer.EvaluateOMPContextHistoryPromotionV1(promptlayer.OMPContextHistoryPromotionInputV1{
		RequestedHistoryMode: promptlayer.OMPContextHistoryModeActiveV1,
		PreviousHistoryMode:  promptlayer.OMPContextHistoryModeShadowV1,
		MemoryMode:           promptlayer.OMPContextMemoryModeOffV1,
		Rows:                 result.Rows,
		Aggregate:            aggregate,
	})
	result.Receipt.PairCount = aggregate.PairCount
	result.Receipt.ABCount = aggregate.ABCount
	result.Receipt.BACount = aggregate.BACount
	result.Receipt.MedianReductionBasisPoints = aggregate.MedianReductionBasisPoints
	result.Receipt.Gates = append([]promptlayer.OMPContextPromotionGateV1(nil), decision.Gates...)
	if !decision.Admitted || decision.EffectiveHistoryMode != promptlayer.OMPContextHistoryModeActiveV1 {
		result.Receipt.Reason = decision.Reason
		return result, fmt.Errorf("OMP context cohort rejected: reason=%s", decision.Reason)
	}

	result.Receipt.Status, result.Receipt.Reason = "evaluated", "candidate-evaluated"
	if input.Log != nil {
		if err := json.NewEncoder(input.Log).Encode(result.Receipt); err != nil {
			result.Receipt.Status, result.Receipt.Reason = "rejected", "receipt-write-failed"
			return result, fmt.Errorf("OMP context cohort rejected: reason=%s", result.Receipt.Reason)
		}
	}
	return result, nil
}

func validatePipelineOMPContextCohortInput(
	input pipelineOMPContextCohortInput,
	receipt *pipelineOMPContextCohortReceipt,
) string {
	if input.LiveOptIn != "explicit-live" {
		return "live-opt-in-required"
	}
	receipt.LiveOptIn = input.LiveOptIn
	if len(input.Tasks) != pipelineOMPContextCohortTaskCount {
		return "exact-task-count-required"
	}
	if !pipelineOMPContextCohortLocatorPattern.MatchString(input.CredentialLocator) {
		return "credential-locator-invalid"
	}
	receipt.CredentialLocator = input.CredentialLocator
	if input.Run == nil {
		return "provider-runner-unavailable"
	}
	seen := make(map[string]struct{}, len(input.Tasks))
	totalPromptBytes := 0
	for index, task := range input.Tasks {
		taskDigest := pipelineOMPContextCohortHash(task.TaskID)
		receipt.TaskDigests = append(receipt.TaskDigests, taskDigest)
		receipt.PromptDigests = append(receipt.PromptDigests, pipelineOMPContextCohortHash(task.Prompt))
		if !pipelineOMPContextCohortTaskIDPattern.MatchString(task.TaskID) {
			return fmt.Sprintf("task-id-invalid-%02d", index+1)
		}
		if task.Prompt == "" || len(task.Prompt) > pipelineOMPContextCohortMaxPrompt {
			return fmt.Sprintf("task-prompt-invalid-%02d", index+1)
		}
		totalPromptBytes += len(task.Prompt)
		if totalPromptBytes > pipelineOMPContextCohortMaxBodies {
			return "task-prompts-exceed-limit"
		}
		if _, exists := seen[task.TaskID]; exists {
			return fmt.Sprintf("task-id-duplicate-%02d", index+1)
		}
		seen[task.TaskID] = struct{}{}
	}
	return ""
}

func pipelineOMPContextCohortOrder(index int) [2]promptlayer.OMPContextCanaryVariantV1 {
	if index%2 == 0 {
		return [2]promptlayer.OMPContextCanaryVariantV1{
			promptlayer.OMPContextCanaryVariantFullV1, promptlayer.OMPContextCanaryVariantOptimizedV1,
		}
	}
	return [2]promptlayer.OMPContextCanaryVariantV1{
		promptlayer.OMPContextCanaryVariantOptimizedV1, promptlayer.OMPContextCanaryVariantFullV1,
	}
}

func pipelineOMPContextCohortRow(
	taskID string,
	variant promptlayer.OMPContextCanaryVariantV1,
	order int,
	observation pipelineOMPContextCohortObservation,
) promptlayer.OMPContextCanaryRowV1 {
	return promptlayer.OMPContextCanaryRowV1{
		TaskID: taskID, Variant: variant, Order: order, Tokens: observation.Tokens,
		IntegrityPassed: observation.IntegrityPassed, SecurityPassed: observation.SecurityPassed,
		QualityScore: observation.QualityScore, FallbackVerified: observation.FallbackVerified,
		RollbackVerified: observation.RollbackVerified,
	}
}

func pipelineOMPContextCohortCallError(callIndex int, taskID, reason string) error {
	return fmt.Errorf(
		"OMP context cohort call=%02d task_digest=%s reason=%s",
		callIndex, pipelineOMPContextCohortHash(taskID), reason,
	)
}

func pipelineOMPContextCohortHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%x", digest[:])
}

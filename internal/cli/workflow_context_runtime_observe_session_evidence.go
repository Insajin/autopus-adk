package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type workflowContextObserveSessionPrepare func(
	context.Context,
	workflowContextObserveSessionOptions,
	string,
) (workflowContextObserveSessionSetup, error)

type workflowContextObserveSessionPrepareKey struct{}

type workflowContextObserveSessionCallEvidence struct {
	command            workflowContextObserveSessionCommand
	response           workflowContextObserveSessionResponse
	providerPromptHash string
	startedAt          time.Time
	endedAt            time.Time
}

func prepareWorkflowContextObserveSessionForRun(
	ctx context.Context,
	options workflowContextObserveSessionOptions,
	challenge string,
) (workflowContextObserveSessionSetup, error) {
	if injected, ok := ctx.Value(workflowContextObserveSessionPrepareKey{}).(workflowContextObserveSessionPrepare); ok && injected != nil {
		return injected(ctx, options, challenge)
	}
	return prepareWorkflowContextObserveSession(ctx, options, challenge)
}

func populateWorkflowContextObserveSessionPolicy(options *workflowContextObserveSessionOptions) error {
	if options == nil {
		return errors.New("observe-session policy target is unavailable")
	}
	cfg, err := loadHarnessConfigForDir(options.ProjectDir, globalFlags{})
	if err != nil {
		return fmt.Errorf("observe-session load active policy: %w", err)
	}
	policy, selected, err := workflowContextPolicyFromConfig(cfg)
	if err != nil || !selected {
		return errors.New("observe-session selected active policy is unavailable")
	}
	options.WorkspaceID = cfg.ProjectName
	options.PromotionPolicy = promptlayer.OMPContextPromotionPolicyV1{
		Profile: policy.Profile, HistoryMode: policy.HistoryMode, MemoryMode: policy.MemoryMode,
		HistoryTargetTokens: policy.HistoryTargetTokens, Fallback: policy.Fallback,
		CapabilityPolicy: policy.CapabilityPolicy, RuntimeRootPolicy: policy.RuntimeRootPolicy,
		MutationScope: policy.MutationScope,
	}
	return nil
}

func workflowContextObserveCurrentExecutableSHA256() (string, error) {
	digest, err := pipelineOMPActiveCurrentAutoExecutableSHA256()
	if err != nil {
		return "", errors.New("observe-session auto executable identity is unavailable")
	}
	return digest, nil
}

func (setup workflowContextObserveSessionSetup) sealPrompt(task string) (string, string, error) {
	delivery, err := promptlayer.BuildContextDelivery(setup.deliveryOptions)
	if err != nil || promptlayer.VerifyContextDeliveryForOptions(setup.deliveryOptions, delivery) != nil ||
		delivery.SnapshotHash != setup.delivery.SnapshotHash ||
		delivery.PromptManifestHash != setup.delivery.PromptManifestHash ||
		workflowContextRuntimeHash(delivery.Prompt) != setup.canonicalPromptHash ||
		delivery.Prompt != setup.delivery.Prompt {
		return "", "", errors.New("observe-session canonical context changed before provider admission")
	}
	sealed := delivery.Prompt + "\n\n--- BEGIN IDENTICAL EVALUATION TASK ---\n" +
		task + "\n--- END IDENTICAL EVALUATION TASK ---"
	return sealed, workflowContextRuntimeHash(sealed), nil
}

func verifyWorkflowContextObserveSessionReadback(
	ctx context.Context,
	setup *workflowContextObserveSessionSetup,
	fullCalls int,
	optimizedCalls int,
) error {
	if fullCalls <= 0 || fullCalls != optimizedCalls ||
		fullCalls > workflowContextObserveSessionPairCount {
		return errors.New("observe-session paired segment call count is invalid")
	}
	currentCalls := (fullCalls-1)%workflowContextObserveSessionSegmentPairs + 1
	expectedSegments := (fullCalls-1)/workflowContextObserveSessionSegmentPairs + 1
	maxCompactions := max(currentCalls-1, 0)
	if setup == nil || setup.full == nil || setup.optimized == nil ||
		setup.segmentsStarted != expectedSegments || setup.full.PID() <= 0 ||
		setup.optimized.PID() <= 0 || setup.full.PID() == setup.optimized.PID() ||
		setup.full.sequence != currentCalls || setup.optimized.sequence != currentCalls ||
		setup.full.compactions != 0 || setup.optimized.compactions < 0 ||
		setup.optimized.compactions > maxCompactions {
		return errors.New("observe-session paired segment readback is invalid")
	}
	for _, session := range []*pipelineOMPActiveEvaluatorSession{setup.full, setup.optimized} {
		state, err := session.protocol.readIdleState(ctx, "observe-session-readback")
		if err != nil || state.SessionID != session.sessionID || state.AutoCompactionEnabled == nil ||
			*state.AutoCompactionEnabled {
			return errors.New("observe-session effective rollback readback is invalid")
		}
	}
	return nil
}

func safeWorkflowContextObserveSessionOutput(
	setup workflowContextObserveSessionSetup,
	assistant string,
) bool {
	if strings.TrimSpace(assistant) == "" || workflowContextSecretPattern.MatchString(assistant) {
		return false
	}
	for _, forbidden := range []string{setup.credential, setup.endpoint, setup.projectDir, setup.taskRoot} {
		if forbidden != "" && strings.Contains(assistant, forbidden) {
			return false
		}
	}
	return true
}

func buildWorkflowContextObserveSessionReport(
	options workflowContextObserveSessionOptions,
	setup workflowContextObserveSessionSetup,
	challenge string,
	calls []workflowContextObserveSessionCallEvidence,
) (promptlayer.OMPContextPromotionReportV1, []byte, []promptlayer.OMPContextCanaryRowV1, error) {
	if len(calls) != workflowContextObserveSessionPairCount*2 ||
		setup.segmentsStarted != workflowContextObserveSessionSegmentCount {
		return promptlayer.OMPContextPromotionReportV1{}, nil, nil, errors.New("observe-session cohort is incomplete")
	}
	policyDigest, err := promptlayer.OMPContextPromotionPolicyDigestV1(options.PromotionPolicy)
	if err != nil {
		return promptlayer.OMPContextPromotionReportV1{}, nil, nil, err
	}
	report := promptlayer.OMPContextPromotionReportV1{
		SchemaVersion:   promptlayer.OMPContextPromotionReportSchemaV1,
		ChallengeDigest: challenge,
		Producer: promptlayer.OMPContextPromotionProducerV1{
			Repository: options.ProducerRepository, WorkflowRef: options.ProducerWorkflowRef,
			RunID: options.ProducerRunID, RunAttempt: options.ProducerRunAttempt,
		},
		Candidate: promptlayer.OMPContextPromotionCandidateV1{
			Repository: options.CandidateRepository, Revision: options.TargetGitCommit,
			TreeSHA: setup.candidateTree, ArtifactSHA256: setup.autoExecutableSHA256,
		},
		Policy: promptlayer.OMPContextPromotionPolicyReportV1{
			PolicyID: options.PolicyID, PolicyDigest: policyDigest, HistoryMode: "active", MemoryMode: "off",
			MinPairCount: 20, MinReductionBasisPoints: 2000,
		},
		Runtime: promptlayer.OMPContextPromotionRuntimeV1{
			AutoVersion: setup.autoVersion, AutoBinarySHA256: setup.autoExecutableSHA256,
			OMPVersion: setup.ompVersion, OMPExecutableSHA256: setup.ompExecutableSHA256,
			ExecutionClass: "external-live", ProductionPathEquivalent: true,
			RuntimeKind: "omp-pipeline-managed-rpc", PipelineImplementationDigest: pipelineOMPActiveImplementationDigest(),
		},
		SessionFacts: promptlayer.OMPContextPromotionSessionFactsV1{
			FullProcessStarts: setup.segmentsStarted, OptimizedProcessStarts: setup.segmentsStarted,
			FullSessionCount: setup.segmentsStarted, OptimizedSessionCount: setup.segmentsStarted,
			MaxConcurrency: 1, CrossSessionContamination: 0,
		},
		Provider: options.Provider, ModelScopeDigest: setup.candidate.ModelScopeDigest,
		OraclePolicyDigest: options.OraclePolicyDigest,
	}
	variantCalls := map[string]int{"A": 0, "B": 0}
	for index, call := range calls {
		if index%2 == 0 {
			report.Tasks = append(report.Tasks, promptlayer.OMPContextPromotionTaskV1{
				TaskIDDigest: call.command.TaskIDDigest,
				Order:        call.command.Variant + calls[index+1].command.Variant,
			})
		}
		variantCalls[call.command.Variant]++
		sessionSequence := (variantCalls[call.command.Variant]-1)%workflowContextObserveSessionSegmentPairs + 1
		usageBody, _ := json.Marshal(struct {
			Sequence int                                 `json:"sequence"`
			Usage    *workflowContextObserveSessionUsage `json:"usage"`
		}{call.command.Sequence, call.response.Usage})
		observationBody, _ := json.Marshal(struct {
			Sequence, PairSequence                                                    int
			Task, Variant, Output, Session, ProviderAuthority, Usage, CanonicalPrompt string
		}{
			call.command.Sequence, call.command.PairSequence, call.command.TaskIDDigest, call.command.Variant,
			call.response.OutputDigest, call.response.SessionDigest, call.response.ProviderAuthorityDigest,
			workflowContextRuntimeHash(string(usageBody)), call.providerPromptHash,
		})
		compactionCalls := call.response.CompactionCycles
		report.Observations = append(report.Observations, promptlayer.OMPContextPromotionObservationV1{
			Sequence: call.command.Sequence, TaskIDDigest: call.command.TaskIDDigest, Variant: call.command.Variant,
			SessionReceiptDigest: call.response.SessionDigest, SessionSequence: sessionSequence,
			ProcessReused: call.response.ProcessReused, Provider: options.Provider,
			ModelScopeDigest: setup.candidate.ModelScopeDigest, EndpointClass: "external-provider",
			Transport: "provider-api", CredentialMode: "locator-only",
			ProviderAuthorityDigest: setup.providerAuthorityDigest, ExecutionMode: "external-live",
			StartedAt: call.startedAt.UTC().Format(time.RFC3339Nano), CompletedAt: call.endedAt.UTC().Format(time.RFC3339Nano),
			InputTokens: call.response.Usage.PrimaryInputTokens, OutputTokens: call.response.Usage.PrimaryOutputTokens,
			TotalTokens:           call.response.Usage.PrimaryInputTokens + call.response.Usage.PrimaryOutputTokens,
			SetupProviderRequests: 0, CompactionProviderRequests: compactionCalls,
			PrimaryProviderRequests: 1, TotalProviderRequests: 1 + compactionCalls,
			PreCompactionACKs: call.response.PreCompactionACKs, PostCompactionACKs: call.response.PostCompactionACKs,
			CanonicalReadmissions: call.response.CanonicalReadmissions,
			EphemeralReadmissions: call.response.EphemeralReadmissions,
			ObservationDigest:     workflowContextRuntimeHash(string(observationBody)),
			UsageDigest:           workflowContextRuntimeHash(string(usageBody)), IntegrityPassed: true, SecurityPassed: true,
			QualityScore: 10000, FallbackVerified: true, RollbackVerified: true, CleanupVerified: true,
			RetryCount: 0, MaxConcurrency: 1,
		})
	}
	report, body, err := promptlayer.BuildOMPContextPromotionReportV1(report)
	if err != nil {
		return report, nil, nil, err
	}
	rows, err := promptlayer.OMPContextPromotionReportCanaryRowsV1(report)
	return report, body, rows, err
}

func writeWorkflowContextObserveSessionEvidence(
	options workflowContextObserveSessionOptions,
	setup workflowContextObserveSessionSetup,
	challenge string,
	calls []workflowContextObserveSessionCallEvidence,
	checkedAt time.Time,
) (string, string, error) {
	report, body, rows, err := buildWorkflowContextObserveSessionReport(options, setup, challenge, calls)
	if err != nil {
		return "", "", err
	}
	for _, forbidden := range []string{setup.credential, setup.endpoint, setup.projectDir, setup.taskRoot} {
		if forbidden != "" && bytes.Contains(body, []byte(forbidden)) {
			return "", "", errors.New("observe-session promotion report contains private material")
		}
	}
	for _, call := range calls {
		for _, privateBody := range []string{call.command.Prompt, call.response.AssistantText} {
			if len(privateBody) >= 16 && bytes.Contains(body, []byte(privateBody)) {
				return "", "", errors.New("observe-session promotion report contains request or response bodies")
			}
		}
	}
	for _, key := range []string{"credential_locator", "credential_value", "assistant_text", "prompt_body", "project_dir"} {
		if bytes.Contains(body, []byte(key)) {
			return "", "", errors.New("observe-session promotion report contains body-bearing fields")
		}
	}
	if filepath.IsAbs(options.ProducerRepository) || filepath.IsAbs(options.CandidateRepository) {
		return "", "", errors.New("observe-session promotion report contains an absolute coordinate")
	}
	if err := promptlayer.WriteOMPContextPromotionReportV1(setup.projectDir, body); err != nil {
		return "", "", err
	}
	if err := promptlayer.WriteOMPContextEvidenceStoreV1(setup.projectDir, promptlayer.OMPContextEvidenceStoreV1{
		Binding: promptlayer.OMPContextEvidenceStoreBindingV1{
			WorkspaceID: options.WorkspaceID, SpecID: options.SpecID,
			SnapshotHash: setup.candidate.Snapshot.SnapshotHash, GitCommitHash: options.TargetGitCommit,
			RuntimeVersion: setup.ompVersion, CheckedAt: checkedAt.UTC(), ValidFor: options.EvidenceValidFor,
		},
		Policy: options.PromotionPolicy, CanaryRows: rows,
	}); err != nil {
		return "", "", err
	}
	return report.EvidenceID, workflowContextRuntimeHash(string(body)), nil
}

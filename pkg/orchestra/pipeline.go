package orchestra

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RoundPresets defines the number of cross-pollination rounds per preset.
var RoundPresets = map[string]int{
	"fast":     0, // independent + judge only
	"standard": 1, // independent + 1 cross-pollinate + judge
	"deep":     2, // independent + 2 cross-pollinate + judge
}

// SubprocessPipelineConfig holds configuration for the subprocess pipeline.
type SubprocessPipelineConfig struct {
	Backend                      ExecutionBackend
	Providers                    []ProviderConfig
	Topic                        string
	PromptData                   PromptData
	Rounds                       int // number of cross-pollination rounds (0=fast, 1=standard, 2=deep)
	Judge                        ProviderConfig
	RequireJudgeFamilySeparation bool
	TimeoutSeconds               int
}

// RunSubprocessPipeline executes the full subprocess debate pipeline:
// prepare → parallel independent → cross-pollinate → judge → merge.
func RunSubprocessPipeline(ctx context.Context, cfg SubprocessPipelineConfig) (*OrchestraResult, error) {
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("pipeline: no providers configured")
	}
	if cfg.Backend == nil {
		return nil, fmt.Errorf("pipeline: backend is nil")
	}
	contractCfg := subprocessPipelineContractConfig(cfg)
	if result, err := preflightJudgeFamilySeparation(contractCfg); err != nil {
		return result, err
	}

	start := time.Now()
	providerNames := make([]string, len(cfg.Providers))
	for i, p := range cfg.Providers {
		providerNames[i] = p.Name
	}
	cpb := NewCrossPollinateBuilder(providerNames)
	pb := cfg.PromptData

	// Phase 1: Independent analysis (parallel)
	schema := &SchemaBuilder{}
	schemaPath, cleanup, err := schema.WriteToFile("debater_r1")
	if err != nil {
		return nil, fmt.Errorf("pipeline: schema: %w", err)
	}
	defer cleanup()

	r1Data, err := withPromptSchema(pb, schema, "debater_r1", cfg.Providers)
	if err != nil {
		return nil, err
	}

	r1Prompt, err := buildPromptBuilder().BuildDebaterR1(r1Data)
	if err != nil {
		return nil, fmt.Errorf("pipeline: debater_r1 prompt: %w", err)
	}

	rawR1Results, r1Failed, err := executeParallel(ctx, cfg.Backend, cfg.Providers, r1Prompt, schemaPath, "debater_r1", 1, cfg.TimeoutSeconds)
	r1Evidence := providerResultsToResponses(rawR1Results)
	if terminal, reasons, transitioned := failedPolicyTransition(r1Failed); transitioned {
		runErr := fmt.Errorf("pipeline: round 1: pane fallback policy %s", terminal)
		return buildSubprocessPolicyTransition(cfg, r1Evidence, r1Failed, nil, start, terminal, reasons), runErr
	}
	if err != nil {
		runErr := fmt.Errorf("pipeline: round 1: %w", err)
		return buildSubprocessParticipantFailure(cfg, r1Failed, nil, start, runErr), runErr
	}
	r1Results, hadR1Skip := activeProviderResults(rawR1Results)
	roundHistory := [][]ProviderResponse{r1Evidence}
	if len(r1Results) == 0 && hadR1Skip {
		reasons := skippedTransitionReasons(r1Evidence)
		return buildSubprocessPolicyTransition(
			cfg, r1Evidence, r1Failed, roundHistory, start, TerminalSkipped, reasons,
		), nil
	}

	allResults := r1Results
	var r2Results []ProviderResult

	// Phase 2: Cross-pollination rounds
	for round := 1; round <= cfg.Rounds; round++ {
		prevAnon := cpb.Anonymize(allResults)
		pb.Round = round + 1
		pb.PreviousRound = round
		pb.PreviousResults = prevAnon
		// REQ-004: fence untrusted participant output with a round-scoped sentinel.
		pb.Sentinel = sentinelForPreviousResults(prevAnon)

		r2Data, schemaErr := withPromptSchema(pb, schema, "debater_r2", cfg.Providers)
		if schemaErr != nil {
			return buildSubprocessParticipantFailure(cfg, r1Failed, roundHistory, start, schemaErr), schemaErr
		}

		r2Prompt, promptErr := buildPromptBuilder().BuildDebaterR2(r2Data)
		if promptErr != nil {
			runErr := fmt.Errorf("pipeline: debater_r2 prompt: %w", promptErr)
			return buildSubprocessParticipantFailure(cfg, r1Failed, roundHistory, start, runErr), runErr
		}

		r2SchemaPath, r2Cleanup, schemaErr := schema.WriteToFile("debater_r2")
		if schemaErr != nil {
			runErr := fmt.Errorf("pipeline: r2 schema: %w", schemaErr)
			return buildSubprocessParticipantFailure(cfg, r1Failed, roundHistory, start, runErr), runErr
		}
		defer r2Cleanup()

		rawRoundResults, roundFailed, roundErr := executeParallel(ctx, cfg.Backend, cfg.Providers, r2Prompt, r2SchemaPath, "debater_r2", round+1, cfg.TimeoutSeconds)
		roundEvidence := providerResultsToResponses(rawRoundResults)
		if terminal, reasons, transitioned := failedPolicyTransition(roundFailed); transitioned {
			r1Failed = append(r1Failed, roundFailed...)
			if len(roundEvidence) > 0 {
				roundHistory = append(roundHistory, roundEvidence)
			}
			runErr := fmt.Errorf("pipeline: round %d: pane fallback policy %s", round+1, terminal)
			return buildSubprocessPolicyTransition(
				cfg, roundEvidence, r1Failed, roundHistory, start, terminal, reasons,
			), runErr
		}
		if roundErr != nil {
			r1Failed = append(r1Failed, roundFailed...)
			runErr := fmt.Errorf("pipeline: round %d: %w", round+1, roundErr)
			return buildSubprocessParticipantFailure(cfg, r1Failed, roundHistory, start, runErr), runErr
		}

		r1Failed = append(r1Failed, roundFailed...)
		roundResults, hadRoundSkip := activeProviderResults(rawRoundResults)
		roundHistory = append(roundHistory, roundEvidence)
		if len(roundResults) == 0 && hadRoundSkip {
			return buildSubprocessPolicyTransition(
				cfg, roundEvidence, r1Failed, roundHistory, start, TerminalSkipped,
				skippedTransitionReasons(roundEvidence),
			), nil
		}
		r2Results = roundResults
		allResults = roundResults
	}

	// Phase 3: Judge synthesis
	judgeAnon := cpb.AnonymizeForJudge(r1Results, r2Results)
	jb := NewJudgeBuilder(buildPromptBuilder())
	// REQ-004: fence untrusted participant output for the judge prompt.
	pb.Sentinel = sentinelForJudgeResults(judgeAnon)
	judgeData, err := withPromptSchema(pb, schema, "judge", []ProviderConfig{cfg.Judge})
	if err != nil {
		return buildSubprocessParticipantFailure(cfg, r1Failed, roundHistory, start, err), err
	}

	judgeReq, err := jb.Build(judgeData, judgeAnon)
	if err != nil {
		runErr := fmt.Errorf("pipeline: judge build: %w", err)
		return buildSubprocessParticipantFailure(cfg, r1Failed, roundHistory, start, runErr), runErr
	}
	judgeReq.Config = cfg.Judge
	judgeReq.Provider = cfg.Judge.Name
	judgeReq.Round = cfg.Rounds + 2
	judgeReq.Timeout = providerExecutionTimeout(cfg.Judge, cfg.TimeoutSeconds)

	judgeSchemaPath, judgeCleanup, err := schema.WriteToFile("judge")
	if err != nil {
		runErr := fmt.Errorf("pipeline: judge schema: %w", err)
		return buildSubprocessParticipantFailure(cfg, r1Failed, roundHistory, start, runErr), runErr
	}
	defer judgeCleanup()
	judgeReq.SchemaPath = judgeSchemaPath

	judgeProgress := NewProgressTracker([]string{cfg.Judge.Name})
	stopJudgeProgress := judgeProgress.StartHeartbeat(ctx, progressHeartbeatInterval)
	defer stopJudgeProgress()
	judgeProgress.MarkRunning(cfg.Judge.Name)
	judgeResp, err := cfg.Backend.Execute(ctx, judgeReq)
	applyProviderRequestEvidence(judgeResp, judgeReq, cfg.Backend.Name())
	if judgeResp != nil && (judgeResp.TerminalState == TerminalSkipped || judgeResp.TerminalState == TerminalBlocked) {
		finalResults := r2Results
		if len(finalResults) == 0 {
			finalResults = r1Results
		}
		responses := providerResultsToResponses(finalResults)
		judgeResp.Provider = cfg.Judge.Name + " (judge)"
		responses = append(responses, *judgeResp)
		reasons := append([]string(nil), judgeResp.DegradedReasons...)
		if judgeResp.TerminalState == TerminalSkipped {
			judgeProgress.MarkDone(cfg.Judge.Name)
			return buildSubprocessPolicyTransition(
				cfg, responses, r1Failed, roundHistory, start, TerminalSkipped, reasons,
			), nil
		}
		judgeProgress.MarkFailed(cfg.Judge.Name)
		if err == nil {
			err = fmt.Errorf("judge pane fallback policy blocked execution")
		}
		judgeFailure := buildFailedProviderWithContext(
			cfg.Judge, judgeResp, err, cfg.TimeoutSeconds, "judge", len(finalResults) > 0,
		)
		judgeFailure.Attempt = judgeReq.Round
		return buildSubprocessPolicyTransition(
			cfg, responses, append(r1Failed, judgeFailure), roundHistory,
			start, TerminalBlocked, reasons,
		), fmt.Errorf("pipeline: judge execute: %w", err)
	}
	if err != nil {
		judgeProgress.MarkFailed(cfg.Judge.Name)
		runErr := fmt.Errorf("pipeline: judge execute: %w", err)
		return buildSubprocessJudgeFailure(cfg, r1Failed, roundHistory, r1Results, r2Results, start, judgeResp, runErr), runErr
	}
	if judgeResp == nil {
		judgeProgress.MarkFailed(cfg.Judge.Name)
		runErr := fmt.Errorf("pipeline: judge returned no response")
		return buildSubprocessJudgeFailure(cfg, r1Failed, roundHistory, r1Results, r2Results, start, nil, runErr), runErr
	}
	if judgeResp.TimedOut {
		judgeProgress.MarkFailed(cfg.Judge.Name)
		runErr := fmt.Errorf("pipeline: judge timed out")
		return buildSubprocessJudgeFailure(cfg, r1Failed, roundHistory, r1Results, r2Results, start, judgeResp, runErr), runErr
	}
	if judgeResp.EmptyOutput || strings.TrimSpace(judgeResp.Output) == "" {
		judgeProgress.MarkFailed(cfg.Judge.Name)
		judgeResp.EmptyOutput = true
		runErr := fmt.Errorf("pipeline: judge returned empty output")
		return buildSubprocessJudgeFailure(cfg, r1Failed, roundHistory, r1Results, r2Results, start, judgeResp, runErr), runErr
	}
	judgeProgress.MarkDone(cfg.Judge.Name)

	// Phase 4: Parse and merge
	parser := &OutputParser{}
	judgeOutput, err := parser.ParseJudge(judgeResp.Output)
	if err != nil {
		runErr := fmt.Errorf("pipeline: judge output invalid: %w", err)
		return buildSubprocessJudgeFailure(cfg, r1Failed, roundHistory, r1Results, r2Results, start, judgeResp, runErr), runErr
	}

	merged := MergeSubprocessResults(judgeOutput, cpb.IdentityMap(), r1Results, r2Results)

	// Build response list for OrchestraResult
	var responses []ProviderResponse
	responses = append(responses, providerResultsToResponses(r1Results)...)
	judgeResp.Provider = cfg.Judge.Name + " (judge)"
	responses = append(responses, *judgeResp)

	result := &OrchestraResult{
		Strategy:        StrategyDebate,
		Responses:       responses,
		Merged:          merged,
		Duration:        time.Since(start),
		Summary:         fmt.Sprintf("subprocess pipeline: %d providers, %d rounds", len(cfg.Providers), cfg.Rounds+1),
		FailedProviders: r1Failed,
		Degraded:        len(r1Failed) > 0,
		RoundHistory:    roundHistory,
		JudgeStatus:     JudgePassed,
	}
	return finalizeOrchestraResultForConfig(result, contractCfg), nil
}

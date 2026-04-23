package orchestra

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	Backend        ExecutionBackend
	Providers      []ProviderConfig
	Topic          string
	PromptData     PromptData
	Rounds         int // number of cross-pollination rounds (0=fast, 1=standard, 2=deep)
	Judge          ProviderConfig
	TimeoutSeconds int
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

	r1Results, r1Failed, err := executeParallel(ctx, cfg.Backend, cfg.Providers, r1Prompt, schemaPath, "debater_r1", 1, cfg.TimeoutSeconds)
	if err != nil {
		return nil, fmt.Errorf("pipeline: round 1: %w", err)
	}

	allResults := r1Results
	var r2Results []ProviderResult

	// Phase 2: Cross-pollination rounds
	for round := 1; round <= cfg.Rounds; round++ {
		prevAnon := cpb.Anonymize(allResults)
		pb.Round = round + 1
		pb.PreviousRound = round
		pb.PreviousResults = prevAnon

		r2Data, schemaErr := withPromptSchema(pb, schema, "debater_r2", cfg.Providers)
		if schemaErr != nil {
			return nil, schemaErr
		}

		r2Prompt, promptErr := buildPromptBuilder().BuildDebaterR2(r2Data)
		if promptErr != nil {
			return nil, fmt.Errorf("pipeline: debater_r2 prompt: %w", promptErr)
		}

		r2SchemaPath, r2Cleanup, schemaErr := schema.WriteToFile("debater_r2")
		if schemaErr != nil {
			return nil, fmt.Errorf("pipeline: r2 schema: %w", schemaErr)
		}
		defer r2Cleanup()

		roundResults, roundFailed, roundErr := executeParallel(ctx, cfg.Backend, cfg.Providers, r2Prompt, r2SchemaPath, "debater_r2", round+1, cfg.TimeoutSeconds)
		if roundErr != nil {
			return nil, fmt.Errorf("pipeline: round %d: %w", round+1, roundErr)
		}

		r1Failed = append(r1Failed, roundFailed...)
		r2Results = roundResults
		allResults = roundResults
	}

	// Phase 3: Judge synthesis
	judgeAnon := cpb.AnonymizeForJudge(r1Results, r2Results)
	jb := NewJudgeBuilder(buildPromptBuilder())
	judgeData, err := withPromptSchema(pb, schema, "judge", []ProviderConfig{cfg.Judge})
	if err != nil {
		return nil, err
	}

	judgeReq, err := jb.Build(judgeData, judgeAnon)
	if err != nil {
		return nil, fmt.Errorf("pipeline: judge build: %w", err)
	}
	judgeReq.Config = cfg.Judge
	judgeReq.Provider = cfg.Judge.Name
	judgeReq.Timeout = providerExecutionTimeout(cfg.Judge, cfg.TimeoutSeconds)

	judgeSchemaPath, judgeCleanup, err := schema.WriteToFile("judge")
	if err != nil {
		return nil, fmt.Errorf("pipeline: judge schema: %w", err)
	}
	defer judgeCleanup()
	judgeReq.SchemaPath = judgeSchemaPath

	judgeResp, err := cfg.Backend.Execute(ctx, judgeReq)
	if err != nil {
		return nil, fmt.Errorf("pipeline: judge execute: %w", err)
	}

	// Phase 4: Parse and merge
	parser := &OutputParser{}
	judgeOutput, err := parser.ParseJudge(judgeResp.Output)
	if err != nil {
		// Non-fatal: use raw output as recommendation if parse fails
		judgeOutput = &JudgeOutput{Recommendation: judgeResp.Output}
	}

	merged := MergeSubprocessResults(judgeOutput, cpb.IdentityMap(), r1Results, r2Results)

	// Build response list for OrchestraResult
	var responses []ProviderResponse
	for _, r := range r1Results {
		responses = append(responses, ProviderResponse{Provider: r.Provider, Output: r.Output})
	}
	judgeResp.Provider = cfg.Judge.Name + " (judge)"
	responses = append(responses, *judgeResp)

	return &OrchestraResult{
		Strategy:        StrategyDebate,
		Responses:       responses,
		Merged:          merged,
		Duration:        time.Since(start),
		Summary:         fmt.Sprintf("subprocess pipeline: %d providers, %d rounds", len(cfg.Providers), cfg.Rounds+1),
		FailedProviders: r1Failed,
	}, nil
}

// executeParallel runs all providers concurrently via the backend.
func executeParallel(
	ctx context.Context,
	backend ExecutionBackend,
	providers []ProviderConfig,
	prompt, schemaPath, role string,
	round int,
	timeoutSeconds int,
) ([]ProviderResult, []FailedProvider, error) {
	type result struct {
		pr  ProviderResult
		err error
		idx int
	}

	results := make([]result, len(providers))
	var wg sync.WaitGroup

	for i, p := range providers {
		wg.Add(1)
		go func(idx int, prov ProviderConfig) {
			defer wg.Done()
			req := ProviderRequest{
				Provider:   prov.Name,
				Prompt:     prompt,
				SchemaPath: schemaPath,
				Role:       role,
				Round:      round,
				Timeout:    providerExecutionTimeout(prov, timeoutSeconds),
				Config:     prov,
			}
			resp, err := backend.Execute(ctx, req)
			if err != nil {
				results[idx] = result{err: err, idx: idx}
				return
			}
			results[idx] = result{
				pr:  ProviderResult{Provider: prov.Name, Output: resp.Output},
				idx: idx,
			}
		}(i, p)
	}
	wg.Wait()

	var successes []ProviderResult
	var failed []FailedProvider
	for _, r := range results {
		if r.err != nil {
			failed = append(failed, FailedProvider{Name: providers[r.idx].Name, Error: r.err.Error()})
		} else {
			successes = append(successes, r.pr)
		}
	}

	if len(successes) == 0 {
		return nil, failed, fmt.Errorf("all %d providers failed", len(providers))
	}
	return successes, failed, nil
}

// buildPromptBuilder creates a PromptBuilder, panicking on error (templates are embedded).
func buildPromptBuilder() *PromptBuilder {
	pb, err := NewPromptBuilder()
	if err != nil {
		panic(fmt.Sprintf("pipeline: failed to create prompt builder: %v", err))
	}
	return pb
}

func withPromptSchema(
	base PromptData,
	schema *SchemaBuilder,
	role string,
	providers []ProviderConfig,
) (PromptData, error) {
	data := base
	if providersSupportCLISchema(providers) {
		data.SchemaMethod = ""
		data.SchemaJSON = ""
		return data, nil
	}

	embedded, err := schema.EmbedInPrompt(role)
	if err != nil {
		return PromptData{}, fmt.Errorf("pipeline: embed %s schema in prompt: %w", role, err)
	}
	data.SchemaMethod = "prompt"
	data.SchemaJSON = embedded
	return data, nil
}

func providersSupportCLISchema(providers []ProviderConfig) bool {
	if len(providers) == 0 {
		return false
	}
	for _, provider := range providers {
		if strings.TrimSpace(provider.SchemaFlag) == "" {
			return false
		}
	}
	return true
}

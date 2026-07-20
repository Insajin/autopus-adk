package orchestra

import (
	"context"
	"fmt"
)

// runPipeline은 프로바이더를 순차적으로 실행하며 이전 출력을 다음 입력에 추가한다.
func runPipeline(ctx context.Context, cfg OrchestraConfig) ([]ProviderResponse, []FailedProvider, error) {
	responses := make([]ProviderResponse, 0, len(cfg.Providers))
	prompt := cfg.Prompt

	for index, provider := range cfg.Providers {
		// Bound each sequential stage by its own per-provider timeout so one
		// slow provider cannot consume the whole orchestration budget and
		// starve the remaining stages.
		perTimeout := providerExecutionTimeout(provider, cfg.TimeoutSeconds)
		stageCtx, stageCancel := context.WithTimeout(ctx, perTimeout)
		response, err := runProvider(stageCtx, provider, prompt)
		stageCancel()
		applyProviderRequestEvidence(response, ProviderRequest{
			Provider: provider.Name, Config: provider, Role: "pipeline_stage", Round: index + 1,
		}, "subprocess")
		if err != nil {
			failure := buildFailedProviderWithContext(
				provider, response, err, cfg.TimeoutSeconds, "pipeline_stage", len(responses) > 0,
			)
			failure.Attempt = index + 1
			failure.ExecutedBackend = "subprocess"
			if response != nil && response.ExecutedBackend != "" {
				failure.ExecutedBackend = response.ExecutedBackend
			}
			return responses, []FailedProvider{failure}, err
		}
		responses = append(responses, *response)
		if response.Output != "" {
			prompt = fmt.Sprintf("%s\n\n이전 단계 결과:\n%s", cfg.Prompt, response.Output)
		}
	}
	return responses, nil, nil
}

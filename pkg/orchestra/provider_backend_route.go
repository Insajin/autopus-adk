package orchestra

import (
	"context"
	"fmt"
)

func runConfiguredProvider(
	ctx context.Context,
	cfg OrchestraConfig,
	provider ProviderConfig,
	prompt string,
	role string,
	round int,
	tracker *ProgressTracker,
) (*ProviderResponse, error) {
	provider = resolveProviderWorkDir(cfg, provider)
	request := ProviderRequest{
		Provider: provider.Name,
		Prompt:   prompt,
		Role:     role,
		Round:    round,
		Timeout:  providerExecutionTimeout(provider, cfg.TimeoutSeconds),
		Config:   provider,
	}
	if provider.Backend == "" {
		response, err := runProviderWithProgress(ctx, provider, prompt, tracker)
		applyProviderRequestEvidence(response, request, "subprocess")
		return response, err
	}

	backend := cfg.ProviderBackends[provider.Backend]
	if backend == nil {
		return nil, fmt.Errorf(
			"provider %s backend %q is not available in this execution path",
			provider.Name,
			provider.Backend,
		)
	}
	if tracker != nil {
		tracker.MarkRunning(provider.Name)
	}
	response, err := backend.Execute(ctx, request)
	applyProviderRequestEvidence(response, request, backend.Name())
	if tracker == nil {
		return response, err
	}
	if err != nil || response == nil || response.TimedOut || response.EmptyOutput {
		tracker.MarkFailed(provider.Name)
	} else {
		tracker.MarkDone(provider.Name)
	}
	return response, err
}

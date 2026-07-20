package orchestra

import "fmt"

func decorateRelayExecutionEvidence(responses []ProviderResponse, cfg OrchestraConfig) []FailedProvider {
	var failed []FailedProvider
	for index := range responses {
		provider := ProviderConfig{Name: responses[index].Provider}
		if index < len(cfg.Providers) {
			provider = cfg.Providers[index]
		}
		applyProviderRequestEvidence(&responses[index], ProviderRequest{
			Provider: provider.Name, Config: provider, Role: "relay_stage", Round: index + 1,
		}, "subprocess")
		if responses[index].ExitCode == 0 && !responses[index].TimedOut && !responses[index].EmptyOutput {
			continue
		}
		failure := buildFailedProviderWithContext(
			provider, &responses[index], fmt.Errorf("relay stage failed"),
			cfg.TimeoutSeconds, "relay_stage", len(responses) > 1,
		)
		failure.Attempt = index + 1
		failure.ExecutedBackend = responses[index].ExecutedBackend
		failed = append(failed, failure)
	}
	return failed
}

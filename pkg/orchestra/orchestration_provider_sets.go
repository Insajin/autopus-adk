package orchestra

import "strings"

func providerConfigNames(providers []ProviderConfig) []string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = appendUniqueName(names, provider.Name)
	}
	return names
}

func firstProviderSet(sets ...[]string) []string {
	for _, set := range sets {
		if len(set) > 0 {
			return cloneProviderNames(set)
		}
	}
	return nil
}

func cloneProviderNames(names []string) []string {
	return append([]string(nil), names...)
}

func attemptedProviderNames(result *OrchestraResult) []string {
	names := make([]string, 0, len(result.Responses)+len(result.FailedProviders))
	for _, round := range result.RoundHistory {
		for _, response := range round {
			if responseWasDispatched(response) {
				names = appendUniqueName(names, trimJudgeRole(response.Provider))
			}
		}
	}
	for _, response := range result.Responses {
		if responseWasDispatched(response) {
			names = appendUniqueName(names, trimJudgeRole(response.Provider))
		}
	}
	for _, failed := range result.FailedProviders {
		if !failed.PreflightFailed {
			names = appendUniqueName(names, failed.Name)
		}
	}
	return names
}

func usableProviderNames(result *OrchestraResult) []string {
	type outcome struct {
		attempt int
		usable  bool
	}
	outcomes := make(map[string]outcome)
	var order []string
	recordResponse := func(response ProviderResponse, fallbackAttempt int) {
		name := trimJudgeRole(response.Provider)
		if name == "" {
			return
		}
		order = appendUniqueName(order, name)
		attempt := response.Attempt
		if attempt == 0 {
			attempt = fallbackAttempt
		}
		usable := responseUsable(response)
		current, exists := outcomes[name]
		if !exists || attempt > current.attempt || (attempt == current.attempt && !usable) {
			outcomes[name] = outcome{attempt: attempt, usable: usable}
		}
	}
	for roundIndex, responses := range result.RoundHistory {
		for _, response := range responses {
			recordResponse(response, roundIndex+1)
		}
	}
	for _, response := range result.Responses {
		recordResponse(response, 1)
	}
	for _, failed := range result.FailedProviders {
		name := trimJudgeRole(failed.Name)
		if name == "" || failed.PreflightFailed {
			continue
		}
		order = appendUniqueName(order, name)
		attempt := failed.Attempt
		if attempt == 0 {
			attempt = 1
		}
		current, exists := outcomes[name]
		if !exists || attempt >= current.attempt {
			outcomes[name] = outcome{attempt: attempt, usable: false}
		}
	}
	var names []string
	for _, name := range order {
		if outcomes[name].usable {
			names = append(names, name)
		}
	}
	return names
}

func countConfiguredUsable(configured, usable []string) int {
	count := 0
	for _, name := range configured {
		if containsProviderName(usable, name) {
			count++
		}
	}
	return count
}

func containsProviderName(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

func failedProviderNames(failed []FailedProvider) []string {
	var names []string
	for _, provider := range failed {
		names = appendUniqueName(names, provider.Name)
	}
	return names
}

func unionProviderNames(sets ...[]string) []string {
	var names []string
	for _, set := range sets {
		for _, name := range set {
			names = appendUniqueName(names, name)
		}
	}
	return names
}

func appendUniqueName(names []string, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return names
	}
	for _, existing := range names {
		if existing == name {
			return names
		}
	}
	return append(names, name)
}

func trimJudgeRole(name string) string {
	return strings.TrimSpace(strings.TrimSuffix(name, " (judge)"))
}

func inferredDispatchCount(result *OrchestraResult) int {
	receipts, _ := buildProviderRunReceipts(result)
	return len(receipts)
}

func majorityQuorum(configured int) int {
	if configured <= 0 {
		return 1
	}
	return configured/2 + 1
}

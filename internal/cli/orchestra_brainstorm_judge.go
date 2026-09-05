package cli

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// @AX:NOTE: [AUTO] the judge provider remains a Round 1 debater; independence comes from a fresh judge session, not family exclusion
func separateBrainstormJudge(providers []orchestra.ProviderConfig, judge string) ([]orchestra.ProviderConfig, string, error) {
	judge = strings.TrimSpace(judge)
	judgeFamily := providerModelFamily(judge)
	for _, provider := range providers {
		family := providerConfigModelFamily(provider)
		if family == "" {
			return nil, "", fmt.Errorf("brainstorm debate: provider %q has no verifiable model family", provider.Name)
		}
		if strings.EqualFold(provider.Name, judge) {
			judgeFamily = family
		}
	}
	if judgeFamily == "" {
		return nil, "", fmt.Errorf("brainstorm debate: judge %q has no verifiable model family", judge)
	}
	if len(providers) < 2 {
		return nil, "", fmt.Errorf("brainstorm debate: at least two configured debaters are required")
	}
	return append([]orchestra.ProviderConfig(nil), providers...), judgeFamily, nil
}

func resolveBrainstormJudgeConfig(
	providers []orchestra.ProviderConfig,
	orchConf *config.OrchestraConf,
	commandName, judge, family, quality, effort string,
) (*orchestra.ProviderConfig, error) {
	for _, provider := range providers {
		if strings.EqualFold(provider.Name, judge) {
			resolved := provider
			resolved.ModelFamily = family
			return &resolved, nil
		}
	}

	var candidates []orchestra.ProviderConfig
	if orchConf != nil {
		candidates = resolveProviders(orchConf, commandName, []string{judge})
	} else {
		candidates = buildProviderConfigsForRuntime([]string{judge}, quality, effort)
	}
	if len(candidates) != 1 || !strings.EqualFold(candidates[0].Name, judge) {
		return nil, fmt.Errorf("brainstorm debate: judge %q configuration is unavailable", judge)
	}
	resolved := candidates[0]
	resolved.ModelFamily = family
	return &resolved, nil
}

func providerConfigModelFamily(provider orchestra.ProviderConfig) string {
	if family := strings.TrimSpace(provider.ModelFamily); family != "" {
		return family
	}
	return providerModelFamily(provider.Name)
}

func providerModelFamily(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(name, "claude"), strings.HasPrefix(name, "anthropic"):
		return "anthropic"
	case strings.HasPrefix(name, "codex"), strings.HasPrefix(name, "openai"), strings.HasPrefix(name, "opencode"):
		return "openai"
	case strings.HasPrefix(name, "gemini"), strings.HasPrefix(name, "google"):
		return "google"
	default:
		return ""
	}
}

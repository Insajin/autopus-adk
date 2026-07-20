package cli

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func separateBrainstormJudge(providers []orchestra.ProviderConfig, judge string) ([]orchestra.ProviderConfig, string, error) {
	judge = strings.TrimSpace(judge)
	judgeFamily := providerModelFamily(judge)
	if judgeFamily == "" {
		return nil, "", fmt.Errorf("brainstorm debate: judge %q has no verifiable model family", judge)
	}

	debaters := make([]orchestra.ProviderConfig, 0, len(providers))
	for _, provider := range providers {
		family := providerModelFamily(provider.Name)
		if family == "" {
			return nil, "", fmt.Errorf("brainstorm debate: provider %q has no verifiable model family", provider.Name)
		}
		if family == judgeFamily {
			continue
		}
		debaters = append(debaters, provider)
	}
	if len(debaters) < 2 {
		return nil, "", fmt.Errorf("brainstorm debate: at least two debaters from families other than judge family %q are required", judgeFamily)
	}
	return debaters, judgeFamily, nil
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

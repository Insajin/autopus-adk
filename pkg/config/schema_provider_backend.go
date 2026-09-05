package config

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// ProviderBackendOMP selects the OMP RPC execution backend.
const ProviderBackendOMP = "omp"

// OMPReviewToolAllowlist is the complete read-only tool set available to OMP reviewers.
var OMPReviewToolAllowlist = []string{"read", "grep", "glob"}

var ompReviewToolSet = map[string]struct{}{
	"read": {},
	"grep": {},
	"glob": {},
}

// EffectiveTools returns a detached, normalized tool list.
func (e ProviderEntry) EffectiveTools() []string {
	tools := append([]string(nil), e.Tools...)
	if len(tools) == 0 {
		tools = append(tools, OMPReviewToolAllowlist...)
	}
	slices.Sort(tools)
	return slices.Compact(tools)
}

func (c *HarnessConfig) validateProviderBackends() error {
	names := make([]string, 0, len(c.Orchestra.Providers))
	for name := range c.Orchestra.Providers {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		entry := c.Orchestra.Providers[name]
		if entry.Backend != "" && entry.Backend != ProviderBackendOMP {
			return fmt.Errorf("provider_backend_invalid: %s", name)
		}
		if entry.Backend == ProviderBackendOMP && !validOMPModelSelector(entry.Model) {
			return fmt.Errorf("provider_model_required: %s", name)
		}
		for _, tool := range entry.Tools {
			if _, ok := ompReviewToolSet[tool]; !ok {
				return fmt.Errorf("provider_tools_invalid: %s", name)
			}
		}
	}
	return nil
}

func validOMPModelSelector(selector string) bool {
	if selector == "" || strings.Count(selector, "/") != 1 || strings.Count(selector, ":") > 1 {
		return false
	}
	if strings.IndexFunc(selector, unicode.IsSpace) >= 0 {
		return false
	}
	provider, modelAndThinking, _ := strings.Cut(selector, "/")
	if strings.Contains(provider, ":") {
		return false
	}
	if provider == "" || modelAndThinking == "" {
		return false
	}
	model, thinking, hasThinking := strings.Cut(modelAndThinking, ":")
	if model == "" {
		return false
	}
	return !hasThinking || thinking != ""
}

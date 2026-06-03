package cli

import (
	"sort"

	"github.com/insajin/autopus-adk/pkg/config"
)

func mergeProviderNames(groups ...[]string) []string {
	seen := make(map[string]struct{})
	var merged []string

	for _, group := range groups {
		for _, name := range group {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			merged = append(merged, name)
		}
	}

	return merged
}

func mergeStringValues(groups ...[]string) []string {
	seen := make(map[string]struct{})
	var merged []string

	for _, group := range groups {
		for _, value := range group {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			merged = append(merged, value)
		}
	}

	return merged
}

func sortedProviderKeys(providers map[string]config.ProviderEntry) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func resolveSpecReviewTimeout(cfg *config.HarnessConfig, requested int) int {
	if requested > 0 {
		return requested
	}
	if cfg != nil && cfg.Orchestra.TimeoutSeconds > 0 {
		return cfg.Orchestra.TimeoutSeconds
	}
	return 120
}

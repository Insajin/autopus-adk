package cli

import "github.com/insajin/autopus-adk/pkg/orchestra"

// specReviewConfiguredNames preserves the policy denominator selected before
// binary/capability resolution. Falling back to resolved providers keeps
// direct/internal callers that predate the configuredNames field compatible.
func specReviewConfiguredNames(configured []string, resolved []orchestra.ProviderConfig) []string {
	if len(configured) > 0 {
		return append([]string(nil), configured...)
	}
	names := make([]string, 0, len(resolved))
	for _, provider := range resolved {
		names = append(names, provider.Name)
	}
	return names
}

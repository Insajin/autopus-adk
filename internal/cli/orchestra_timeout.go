package cli

import (
	"fmt"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

type ResolvedProviderTimeout struct {
	Provider string        `json:"provider"`
	Duration time.Duration `json:"duration"`
	Source   string        `json:"source"`
}

type ResolvedOrchestraTimeout struct {
	Seconds   int                       `json:"seconds"`
	Source    string                    `json:"source"`
	Providers []ResolvedProviderTimeout `json:"providers,omitempty"`
}

func resolveOrchestraTimeout(conf *config.OrchestraConf, requestedTimeout int, timeoutChanged bool, providers []orchestra.ProviderConfig) ResolvedOrchestraTimeout {
	resolved := ResolvedOrchestraTimeout{
		Seconds: resolveCommandTimeout(conf, requestedTimeout, timeoutChanged),
		Source:  "command default",
	}
	if conf != nil && conf.TimeoutSeconds > 0 && !timeoutChanged {
		resolved.Source = "autopus.yaml orchestra.timeout_seconds"
	}
	if timeoutChanged && requestedTimeout > 0 {
		resolved.Source = "flag --timeout"
	}

	fallback := time.Duration(resolved.Seconds) * time.Second
	for _, provider := range providers {
		detail := ResolvedProviderTimeout{
			Provider: provider.Name,
			Duration: fallback,
			Source:   resolved.Source,
		}
		if conf != nil {
			if entry, ok := conf.Providers[provider.Name]; ok && entry.Subprocess.Timeout > 0 {
				detail.Duration = time.Duration(entry.Subprocess.Timeout) * time.Second
				detail.Source = fmt.Sprintf("autopus.yaml orchestra.providers.%s.subprocess.timeout", provider.Name)
			}
		}
		if detail.Source == resolved.Source && provider.StartupTimeout > 0 {
			detail.Duration = provider.StartupTimeout
			detail.Source = fmt.Sprintf("built-in provider default (%s)", provider.Name)
		}
		resolved.Providers = append(resolved.Providers, detail)
	}
	return resolved
}

package cli

import (
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// loadOrchestraConfig loads the orchestra configuration from autopus.yaml
// located in the current working directory.
func loadOrchestraConfig() (*config.OrchestraConf, error) {
	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}
	return &cfg.Orchestra, nil
}

// resolveProviders converts config providers to orchestra.ProviderConfig slice.
// Priority order: CLI flags > command-specific config > all global config providers.
//
// For each resolved provider name:
//   - If the name exists in conf.Providers, use its Binary, Args, and PromptViaArgs.
//   - If not found, fall back to: Binary=name, Args=[], PromptViaArgs=false.
func resolveProviders(conf *config.OrchestraConf, commandName string, flagProviders []string) []orchestra.ProviderConfig {
	// Determine provider names by priority
	names := resolveProviderNames(conf, commandName, flagProviders)

	result := make([]orchestra.ProviderConfig, 0, len(names))
	for _, name := range names {
		entry, ok := conf.Providers[name]
		if !ok {
			// Unknown provider: use name as binary with no args
			result = append(result, orchestra.ProviderConfig{
				Name:          name,
				Binary:        name,
				Args:          []string{},
				PromptViaArgs: false,
			})
			continue
		}
		result = append(result, orchestra.ProviderConfig{
			Name:          name,
			Binary:        entry.Binary,
			Args:          entry.Args,
			PromptViaArgs: entry.PromptViaArgs,
		})
	}
	return result
}

// resolveProviderNames returns provider names based on priority:
// 1. flagProviders (non-empty CLI flag)
// 2. conf.Commands[commandName].Providers (command-specific config)
// 3. all keys from conf.Providers (global fallback)
func resolveProviderNames(conf *config.OrchestraConf, commandName string, flagProviders []string) []string {
	if len(flagProviders) > 0 {
		return flagProviders
	}

	if cmd, ok := conf.Commands[commandName]; ok && len(cmd.Providers) > 0 {
		return cmd.Providers
	}

	// Collect all configured provider names
	names := make([]string, 0, len(conf.Providers))
	for name := range conf.Providers {
		names = append(names, name)
	}
	return names
}

// resolveStrategy determines the strategy to use.
// Priority order: CLI flag > command-specific config > global default > "consensus".
func resolveStrategy(conf *config.OrchestraConf, commandName string, flagStrategy string) string {
	if flagStrategy != "" {
		return flagStrategy
	}

	if cmd, ok := conf.Commands[commandName]; ok && cmd.Strategy != "" {
		return cmd.Strategy
	}

	if conf.DefaultStrategy != "" {
		return conf.DefaultStrategy
	}

	return "consensus"
}

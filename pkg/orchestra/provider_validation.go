package orchestra

import (
	"fmt"
	"strings"
)

func validateSafeArtifactName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	if name != sanitizeProviderName(name) {
		return fmt.Errorf("%s %q is unsafe", kind, name)
	}
	return nil
}

func validateHookSessionID(sessionID string) error {
	if err := validateSafeArtifactName("hook session ID", sessionID); err != nil {
		return err
	}
	if sessionID != strings.ToLower(sessionID) {
		return fmt.Errorf("hook session ID %q must use canonical lowercase", sessionID)
	}
	return nil
}

func providerCanonicalName(name string) string {
	return strings.ToLower(name)
}

func validateProviderConfigs(providers []ProviderConfig) error {
	rawNames := make(map[string]int, len(providers))
	canonicalNames := make(map[string]string, len(providers))
	for index, provider := range providers {
		kind := fmt.Sprintf("provider[%d] name", index)
		if err := validateSafeArtifactName(kind, provider.Name); err != nil {
			return err
		}
		if first, exists := rawNames[provider.Name]; exists {
			return fmt.Errorf("duplicate raw provider name %q at indexes %d and %d", provider.Name, first, index)
		}
		rawNames[provider.Name] = index

		canonical := providerCanonicalName(provider.Name)
		if first, exists := canonicalNames[canonical]; exists {
			return fmt.Errorf("duplicate canonical provider name %q conflicts with %q", provider.Name, first)
		}
		canonicalNames[canonical] = provider.Name
	}
	return nil
}

func validateOrchestraProviderConfig(cfg OrchestraConfig) error {
	if err := validateProviderConfigs(cfg.Providers); err != nil {
		return err
	}
	if cfg.HookMode {
		if err := validateHookSessionID(cfg.SessionID); err != nil {
			return err
		}
	}
	if cfg.JudgeProvider != "" {
		if err := validateSafeArtifactName("judge provider name", cfg.JudgeProvider); err != nil {
			return err
		}
	}
	if cfg.JudgeConfig != nil && cfg.JudgeConfig.Name != "" {
		if err := validateSafeArtifactName("judge config name", cfg.JudgeConfig.Name); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderRequest(req ProviderRequest) error {
	if err := validateSafeArtifactName("provider request name", req.Provider); err != nil {
		return err
	}
	return validateSafeArtifactName("provider config name", req.Config.Name)
}

func validateSubprocessPipelineProviders(providers []ProviderConfig, judge ProviderConfig) error {
	if err := validateProviderConfigs(providers); err != nil {
		return err
	}
	return validateSafeArtifactName("judge provider name", judge.Name)
}

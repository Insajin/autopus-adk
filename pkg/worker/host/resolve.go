package host

import (
	"fmt"
	"strings"

	worker "github.com/insajin/autopus-adk/pkg/worker"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// Input defines typed host assembly inputs independent from Cobra state.
type Input struct {
	ConfigPath      string
	MCPConfigPath   string
	CredentialsPath string
}

// RuntimeConfig is the resolved worker host runtime configuration.
type RuntimeConfig struct {
	BackendURL           string
	WorkspaceID          string
	ProviderName         string
	ProviderAdapter      adapter.ProviderAdapter
	AuthToken            string
	CredentialStore      setup.CredentialStore
	WorkDir              string
	MCPConfigPath        string
	CredentialsPath      string
	RequestedConcurrency int
	MaxConcurrency       int
	WorktreeIsolation    bool
	KnowledgeSync        bool
	KnowledgeDir         string
	MemoryAgentID        string
	WorkerName           string
	Warnings             []string
}

// LoopConfig converts the resolved runtime config into the shared WorkerLoop config.
func (cfg RuntimeConfig) LoopConfig() worker.LoopConfig {
	return worker.LoopConfig{
		BackendURL:        cfg.BackendURL,
		WorkerName:        cfg.WorkerName,
		MemoryAgentID:     cfg.MemoryAgentID,
		Skills:            []string{"coding", "review"},
		Provider:          cfg.ProviderAdapter,
		MCPConfig:         cfg.MCPConfigPath,
		WorkDir:           cfg.WorkDir,
		AuthToken:         cfg.AuthToken,
		CredentialsPath:   cfg.CredentialsPath,
		CredentialStore:   cfg.CredentialStore,
		WorkspaceID:       cfg.WorkspaceID,
		MaxConcurrency:    cfg.MaxConcurrency,
		WorktreeIsolation: cfg.WorktreeIsolation,
		KnowledgeSync:     cfg.KnowledgeSync,
		KnowledgeDir:      cfg.KnowledgeDir,
	}
}

// ResolveRuntime assembles the shared worker host runtime configuration.
func ResolveRuntime(input Input) (RuntimeConfig, error) {
	cfg, err := loadWorkerConfig(input.ConfigPath)
	if err != nil {
		return RuntimeConfig{}, &Error{
			Code:    ErrorConfigLoad,
			Message: "worker configuration is unavailable; run 'auto worker setup' first",
			Err:     err,
		}
	}

	credentialsPath := strings.TrimSpace(input.CredentialsPath)
	if credentialsPath == "" {
		credentialsPath = setup.DefaultCredentialsPath()
	}

	credStore, warn := resolveCredentialStore(input.CredentialsPath, credentialsPath)
	authToken, err := setup.LoadAuthTokenFromPath(input.CredentialsPath)
	if err != nil {
		return RuntimeConfig{}, &Error{
			Code:    ErrorAuthLoad,
			Message: "worker auth token could not be loaded",
			Err:     err,
		}
	}
	if strings.TrimSpace(authToken) == "" {
		return RuntimeConfig{}, &Error{
			Code:    ErrorAuthMissing,
			Message: "worker auth token is missing; run 'auto worker setup' first",
		}
	}

	providerName := ResolveProviderName(cfg.Providers)
	if providerName == "" {
		return RuntimeConfig{}, &Error{
			Code:    ErrorProviderMissing,
			Message: "no worker provider is configured; run 'auto worker setup' to detect providers",
		}
	}
	providerAdapter, err := resolveProviderAdapter(providerName)
	if err != nil {
		return RuntimeConfig{}, &Error{
			Code:    ErrorProviderResolve,
			Message: fmt.Sprintf("worker provider %q is not available", providerName),
			Err:     err,
		}
	}

	workDir := strings.TrimSpace(cfg.WorkDir)
	if workDir == "" {
		workDir = "."
	}

	mcpConfigPath := strings.TrimSpace(input.MCPConfigPath)
	if mcpConfigPath == "" {
		mcpConfigPath = setup.DefaultMCPConfigPath()
	}

	runtimeCfg := RuntimeConfig{
		BackendURL:           cfg.BackendURL,
		WorkspaceID:          cfg.WorkspaceID,
		ProviderName:         providerName,
		ProviderAdapter:      providerAdapter,
		AuthToken:            authToken,
		CredentialStore:      credStore,
		WorkDir:              workDir,
		MCPConfigPath:        mcpConfigPath,
		CredentialsPath:      credentialsPath,
		RequestedConcurrency: cfg.Concurrency,
		MaxConcurrency:       EffectiveConcurrency(providerName, cfg.Concurrency),
		WorktreeIsolation:    cfg.WorktreeIsolation || EffectiveConcurrency(providerName, cfg.Concurrency) > 1,
		KnowledgeSync:        true,
		KnowledgeDir:         cfg.KnowledgeDir,
		MemoryAgentID:        cfg.MemoryAgentID,
		WorkerName:           fmt.Sprintf("adk-worker-%s", providerName),
	}
	if warn != "" {
		runtimeCfg.Warnings = append(runtimeCfg.Warnings, warn)
	}
	return runtimeCfg, nil
}

// ResolveProviderName selects the first configured or installed provider.
func ResolveProviderName(providers []string) string {
	for _, name := range providers {
		if authenticated, _ := setup.CheckProviderAuth(name); authenticated {
			return name
		}
	}
	if len(providers) > 0 {
		return providers[0]
	}
	for _, candidate := range setup.DetectProviders() {
		if candidate.Installed {
			if authenticated, _ := setup.CheckProviderAuth(candidate.Name); authenticated {
				return candidate.Name
			}
		}
	}
	for _, candidate := range setup.DetectProviders() {
		if candidate.Installed {
			return candidate.Name
		}
	}
	return ""
}

// EffectiveConcurrency applies provider-specific concurrency guards.
func EffectiveConcurrency(providerName string, requested int) int {
	if strings.EqualFold(providerName, "codex") && requested > 1 {
		return 1
	}
	return requested
}

func loadWorkerConfig(path string) (*setup.WorkerConfig, error) {
	if strings.TrimSpace(path) == "" {
		return setup.LoadWorkerConfig()
	}
	return setup.LoadWorkerConfigFrom(path)
}

func resolveCredentialStore(overridePath, resolvedPath string) (setup.CredentialStore, string) {
	if strings.TrimSpace(overridePath) != "" {
		return setup.NewPathCredentialStore(resolvedPath), ""
	}
	return setup.NewCredentialStore()
}

func resolveProviderAdapter(name string) (adapter.ProviderAdapter, error) {
	registry := adapter.NewRegistry()
	registry.Register(&adapter.ClaudeAdapter{})
	registry.Register(&adapter.CodexAdapter{})
	registry.Register(&adapter.GeminiAdapter{})
	return registry.Get(name)
}

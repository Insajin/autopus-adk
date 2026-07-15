package host

import (
	"fmt"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	worker "github.com/insajin/autopus-adk/pkg/worker"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// @AX:NOTE: [AUTO] explicit env override permits root-worktree fallback only when a human-provided reason is present.
const worktreeFallbackOverrideReasonEnv = "AUTOPUS_WORKTREE_FALLBACK_OVERRIDE_REASON"

// Input defines typed host assembly inputs independent from Cobra state.
type Input struct {
	ConfigPath      string
	MCPConfigPath   string
	CredentialsPath string
}

// RuntimeConfig is the resolved worker host runtime configuration.
type RuntimeConfig struct {
	BackendURL                     string
	WorkspaceID                    string
	ProviderName                   string
	ProviderAdapter                adapter.ProviderAdapter
	AuthToken                      string
	CredentialStore                setup.CredentialStore
	WorkDir                        string
	MCPConfigPath                  string
	CredentialsPath                string
	RequestedConcurrency           int
	MaxConcurrency                 int
	WorktreeIsolation              bool
	WorktreeFallbackOverrideReason string
	KnowledgeSync                  bool
	KnowledgeDir                   string
	MemoryAgentID                  string
	WorkerName                     string
	Warnings                       []string
}

// LoopConfig converts the resolved runtime config into the shared WorkerLoop config.
func (cfg RuntimeConfig) LoopConfig() worker.LoopConfig {
	telemetryRoot := strings.TrimSpace(cfg.WorkDir)
	if telemetryRoot == "" {
		telemetryRoot = "."
	}
	return worker.LoopConfig{
		BackendURL:                     cfg.BackendURL,
		WorkerName:                     cfg.WorkerName,
		MemoryAgentID:                  cfg.MemoryAgentID,
		Skills:                         []string{"coding", "review"},
		Providers:                      []string{cfg.ProviderName},
		Provider:                       cfg.ProviderAdapter,
		MCPConfig:                      cfg.MCPConfigPath,
		WorkDir:                        cfg.WorkDir,
		AuthToken:                      cfg.AuthToken,
		CredentialsPath:                cfg.CredentialsPath,
		CredentialStore:                cfg.CredentialStore,
		WorkspaceID:                    cfg.WorkspaceID,
		MaxConcurrency:                 cfg.MaxConcurrency,
		WorktreeIsolation:              cfg.WorktreeIsolation,
		WorktreeFallbackOverrideReason: runtimeWorktreeFallbackOverrideReason(),
		KnowledgeSync:                  cfg.KnowledgeSync,
		KnowledgeDir:                   cfg.KnowledgeDir,
		RecordAgentRun: func(run telemetry.AgentRun) error {
			return telemetry.AppendAgentRun(telemetryRoot, run)
		},
	}
}

func runtimeWorktreeFallbackOverrideReason() string {
	return strings.TrimSpace(os.Getenv(worktreeFallbackOverrideReasonEnv))
}

// ResolveRuntime assembles the shared worker host runtime configuration.
func ResolveRuntime(input Input) (RuntimeConfig, error) {
	cfg, err := loadWorkerConfig(input.ConfigPath)
	if err != nil {
		return RuntimeConfig{}, &Error{
			Code:    ErrorConfigLoad,
			Message: "desktop runtime configuration is unavailable; run 'auto connect' or complete desktop auth first",
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
			Message: "desktop runtime auth token is missing; run 'auto connect' or complete desktop auth first",
		}
	}
	memoryAgentID, memoryWarnings := resolveRuntimeMemoryAgentID(cfg.BackendURL, authToken, cfg.WorkspaceID, cfg.MemoryAgentID)

	providerName := ResolveProviderName(cfg.Providers)
	if providerName == "" {
		return RuntimeConfig{}, &Error{
			Code:    ErrorProviderMissing,
			Message: "no worker provider is configured; run 'auto connect' first and configure a local provider (legacy local-host setup remains available via 'auto worker setup')",
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
		BackendURL:                     cfg.BackendURL,
		WorkspaceID:                    cfg.WorkspaceID,
		ProviderName:                   providerName,
		ProviderAdapter:                providerAdapter,
		AuthToken:                      authToken,
		CredentialStore:                credStore,
		WorkDir:                        workDir,
		MCPConfigPath:                  mcpConfigPath,
		CredentialsPath:                credentialsPath,
		RequestedConcurrency:           cfg.Concurrency,
		MaxConcurrency:                 EffectiveConcurrency(providerName, cfg.Concurrency),
		WorktreeIsolation:              cfg.WorktreeIsolation || EffectiveConcurrency(providerName, cfg.Concurrency) > 1,
		WorktreeFallbackOverrideReason: runtimeWorktreeFallbackOverrideReason(),
		KnowledgeSync:                  true,
		KnowledgeDir:                   cfg.KnowledgeDir,
		MemoryAgentID:                  memoryAgentID,
		WorkerName:                     fmt.Sprintf("adk-worker-%s", providerName),
	}
	if warn != "" {
		runtimeCfg.Warnings = append(runtimeCfg.Warnings, warn)
	}
	runtimeCfg.Warnings = append(runtimeCfg.Warnings, memoryWarnings...)
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
	// @AX:NOTE: [AUTO] concurrency defaults encode worker safety policy: codex stays sequential, other providers default to 5 slots.
	if requested <= 0 {
		if strings.EqualFold(providerName, "codex") {
			return 1
		}
		return 5
	}
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

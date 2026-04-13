package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	worker "github.com/insajin/autopus-adk/pkg/worker"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// runWorkerForeground loads config and credentials, then starts the WorkerLoop.
// It blocks until SIGINT/SIGTERM or the context is cancelled.
func runWorkerForeground() error {
	cfg, err := setup.LoadWorkerConfig()
	if err != nil {
		return fmt.Errorf("load worker config: %w (run 'auto worker setup' first)", err)
	}

	// V4 fix: LoadAuthToken reads the correct token based on auth_type
	// (api_key → Worker API Key, jwt → access_token).
	authToken, err := setup.LoadAuthToken()
	if err != nil {
		return fmt.Errorf("load auth token: %w", err)
	}
	if authToken == "" {
		return fmt.Errorf("no auth credentials found (run 'auto worker setup' first)")
	}

	// Resolve provider adapter from config.
	providerName := resolveProvider(cfg.Providers)
	if providerName == "" {
		return fmt.Errorf("no provider configured (run 'auto worker setup' to detect providers)")
	}
	provider, err := resolveProviderAdapter(providerName)
	if err != nil {
		return fmt.Errorf("provider %q: %w", providerName, err)
	}

	log.Printf("[worker] starting: provider=%s workspace=%s backend=%s",
		providerName, cfg.WorkspaceID, cfg.BackendURL)

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = "."
	}

	loopCfg := worker.LoopConfig{
		BackendURL:        cfg.BackendURL,
		WorkerName:        fmt.Sprintf("adk-worker-%s", providerName),
		MemoryAgentID:     cfg.MemoryAgentID,
		Skills:            []string{"coding", "review"},
		Provider:          provider,
		MCPConfig:         setup.DefaultMCPConfigPath(),
		WorkDir:           workDir,
		AuthToken:         authToken,
		CredentialsPath:   setup.DefaultCredentialsPath(),
		WorkspaceID:       cfg.WorkspaceID,
		MaxConcurrency:    cfg.Concurrency,
		KnowledgeSync:     true, // enable KH file sync when WorkspaceID is set
		KnowledgeDir:      cfg.KnowledgeDir,
		KnowledgeSourceID: cfg.KnowledgeSourceID,
	}

	wl := worker.NewWorkerLoop(loopCfg)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := wl.Start(ctx); err != nil {
		return fmt.Errorf("worker start: %w", err)
	}

	<-ctx.Done()
	log.Println("[worker] shutting down...")
	_ = wl.Close()
	return nil
}

// resolveProvider picks the first available provider from config.
func resolveProvider(providers []string) string {
	if len(providers) > 0 {
		return providers[0]
	}
	// Fallback: detect installed providers.
	for _, p := range setup.DetectProviders() {
		if p.Installed {
			return p.Name
		}
	}
	return ""
}

// resolveProviderAdapter creates a ProviderAdapter for the given name.
func resolveProviderAdapter(name string) (adapter.ProviderAdapter, error) {
	reg := adapter.NewRegistry()
	reg.Register(&adapter.ClaudeAdapter{})
	reg.Register(&adapter.CodexAdapter{})
	reg.Register(&adapter.GeminiAdapter{})
	return reg.Get(name)
}

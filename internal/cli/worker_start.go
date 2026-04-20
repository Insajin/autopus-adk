package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/insajin/autopus-adk/pkg/worker/host"
)

// runWorkerForeground resolves the shared host assembly and blocks until shutdown.
func runWorkerForeground() error {
	runtime, err := host.NewRuntime(host.Input{})
	if err != nil {
		return err
	}

	cfg := runtime.Config()
	for _, warning := range cfg.Warnings {
		log.Printf("[worker] credential store warning: %s", warning)
	}
	log.Printf("[worker] starting: provider=%s workspace=%s backend=%s",
		cfg.ProviderName, cfg.WorkspaceID, cfg.BackendURL)
	if cfg.MaxConcurrency != cfg.RequestedConcurrency {
		log.Printf("[worker] provider=%s clamps concurrency from %d to %d for stable task execution",
			cfg.ProviderName, cfg.RequestedConcurrency, cfg.MaxConcurrency)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := runtime.Start(ctx); err != nil {
		return fmt.Errorf("worker start: %w", err)
	}

	<-ctx.Done()
	log.Println("[worker] shutting down...")
	_ = runtime.Close()
	return nil
}

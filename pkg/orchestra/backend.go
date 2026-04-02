package orchestra

import (
	"context"
	"time"
)

// ExecutionBackend abstracts how a provider is executed.
// PaneBackend runs via terminal panes; SubprocessBackend runs as a child process.
type ExecutionBackend interface {
	// Execute runs a single provider and returns its response.
	Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error)
	// Name returns the backend identifier (e.g., "pane", "subprocess").
	Name() string
}

// ProviderRequest is the input for ExecutionBackend.Execute.
type ProviderRequest struct {
	Provider   string         // provider name
	Prompt     string         // prompt text to send
	SchemaPath string         // path to JSON schema file (subprocess mode)
	Role       string         // role descriptor for the provider
	Round      int            // current round number (debate/multi-round)
	Timeout    time.Duration  // per-provider timeout
	Config     ProviderConfig // full provider configuration
}

// PaneBackend implements ExecutionBackend by delegating to runProvider().
type PaneBackend struct{}

// NewPaneBackend creates a PaneBackend instance.
func NewPaneBackend() *PaneBackend {
	return &PaneBackend{}
}

// Execute runs the provider via the existing runProvider function.
func (b *PaneBackend) Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	return runProvider(ctx, req.Config, req.Prompt)
}

// Name returns "pane".
func (b *PaneBackend) Name() string {
	return "pane"
}

// SelectBackend chooses the appropriate ExecutionBackend based on config.
// SubprocessBackend is selected when SubprocessMode is true or Terminal is nil.
// Otherwise PaneBackend is used.
func SelectBackend(cfg OrchestraConfig) ExecutionBackend {
	if cfg.SubprocessMode || cfg.Terminal == nil {
		return NewSubprocessBackendImpl()
	}
	return NewPaneBackend()
}

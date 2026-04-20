package host

import (
	"context"

	worker "github.com/insajin/autopus-adk/pkg/worker"
)

// Runtime is the shared host assembly entry point for foreground and sidecar hosts.
type Runtime struct {
	config RuntimeConfig
	loop   *worker.WorkerLoop
}

// NewRuntime resolves config and assembles a reusable WorkerLoop host.
func NewRuntime(input Input) (*Runtime, error) {
	cfg, err := ResolveRuntime(input)
	if err != nil {
		return nil, err
	}
	return &Runtime{
		config: cfg,
		loop:   worker.NewWorkerLoop(cfg.LoopConfig()),
	}, nil
}

// Config returns the resolved runtime configuration.
func (r *Runtime) Config() RuntimeConfig {
	return r.config
}

// AddObserver attaches a host-neutral observer to the shared worker loop.
func (r *Runtime) AddObserver(observer worker.HostObserver) {
	r.loop.AddHostObserver(observer)
}

// Start launches the shared worker loop.
func (r *Runtime) Start(ctx context.Context) error {
	return r.loop.Start(ctx)
}

// Close stops the shared worker loop.
func (r *Runtime) Close() error {
	return r.loop.Close()
}

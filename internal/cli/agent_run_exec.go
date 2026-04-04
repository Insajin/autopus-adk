package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/stream"
)

// execResult holds the outcome of running a provider subprocess.
type execResult struct {
	Status     string
	CostUSD    float64
	DurationMS int64
	SessionID  string
	Output     string
}

// buildDefaultRegistry creates a registry with all known provider adapters.
func buildDefaultRegistry() *adapter.Registry {
	reg := adapter.NewRegistry()
	reg.Register(adapter.NewClaudeAdapter())
	reg.Register(adapter.NewCodexAdapter())
	reg.Register(adapter.NewGeminiAdapter())
	return reg
}

// executeAgentTask resolves the adapter, spawns subprocess, parses stream, returns result.
func executeAgentTask(ctx context.Context, reg *adapter.Registry, providerName string, taskCfg adapter.TaskConfig) (execResult, error) {
	prov, err := reg.Get(providerName)
	if err != nil {
		return execResult{}, fmt.Errorf("resolve provider %q: %w", providerName, err)
	}

	cmd := prov.BuildCommand(ctx, taskCfg)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return execResult{}, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return execResult{}, fmt.Errorf("stdout pipe: %w", err)
	}

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return execResult{}, fmt.Errorf("start subprocess: %w", err)
	}

	// Write prompt via stdin then close to signal EOF to the subprocess.
	_, _ = io.Copy(stdinPipe, strings.NewReader(taskCfg.Prompt))
	stdinPipe.Close()

	// Parse stream output and capture the last result event.
	parser := stream.NewParser(stdout)
	var lastResult adapter.TaskResult
	hasResult := false
	for {
		evt, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[agent-run] stream parse error: %v", err)
			continue
		}
		if evt.Type == "result" {
			adapterEvt := adapter.StreamEvent{
				Type:    evt.Type,
				Subtype: evt.Subtype,
				Data:    evt.Raw,
			}
			lastResult = prov.ExtractResult(adapterEvt)
			hasResult = true
		}
	}

	waitErr := cmd.Wait()
	durationMS := time.Since(startTime).Milliseconds()

	if waitErr != nil && !hasResult {
		return execResult{Status: "failed", DurationMS: durationMS},
			fmt.Errorf("subprocess exited with error: %w", waitErr)
	}

	if !hasResult {
		return execResult{Status: "failed", DurationMS: durationMS},
			fmt.Errorf("no result event from subprocess")
	}

	return execResult{
		Status:     "success",
		CostUSD:    lastResult.CostUSD,
		DurationMS: durationMS,
		SessionID:  lastResult.SessionID,
		Output:     lastResult.Output,
	}, nil
}

package omp

import (
	"bytes"
	"context"
	"encoding/json"
)

const (
	ompReadinessNegotiateID   = "readiness-negotiate"
	ompReadinessStateID       = "readiness-state"
	ompReadinessCommandsID    = "readiness-commands"
	ompReadinessSubscribeID   = "readiness-subscribe"
	ompReadinessUnsubscribeID = "readiness-unsubscribe"
	// @AX:NOTE [AUTO]: this selector satisfies OMP RPC bootstrap parsing only; the readiness path never invokes its provider.
	ompReadinessBootstrapModel = "openai-codex/gpt-5.6-sol"
)

func ompProviderFreeRPCInput() []byte {
	frames := []map[string]any{
		{"id": ompReadinessNegotiateID, "type": "negotiate_protocol", "protocolVersion": 2},
		{"id": ompReadinessStateID, "type": "get_state"},
		{"id": ompReadinessCommandsID, "type": "get_available_commands"},
		{"id": ompReadinessSubscribeID, "type": "set_subagent_subscription", "level": "progress"},
		{"id": ompReadinessUnsubscribeID, "type": "set_subagent_subscription", "level": "off"},
	}
	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	for _, frame := range frames {
		_ = encoder.Encode(frame)
	}
	return input.Bytes()
}

func ompProviderFreeRPCArgs(root string) []string {
	return []string{
		"--mode", "rpc",
		"--no-session",
		"--cwd", root,
		"--model", ompReadinessBootstrapModel,
		"--tools", "task,hub,todo",
		"--no-extensions",
		"--no-rules",
		"--no-lsp",
		"--no-pty",
	}
}

func runOMPReadinessProviderFreeProbe(
	parent context.Context,
	opts OMPReadinessOptions,
	runner commandOMPProbeRunner,
) ompProbeResult {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, opts.Timeout)
	defer cancel()
	input := ompProviderFreeRPCInput()
	// @AX:NOTE [AUTO]: 2 KiB bounds the deterministic five-frame readiness request before process launch.
	if len(input) > 2048 {
		return ompProbeResult{reason: "output_invalid"}
	}
	output, err := runner.runRPC(
		ctx, opts.Executable, opts.Root, input, ompProviderFreeRPCArgs(opts.Root)...,
	)
	if err != nil {
		return ompProbeResult{output: output, reason: classifyOMPProbeError(ctx, err)}
	}
	if len(output) > opts.MaxOutput {
		return ompProbeResult{reason: "output_oversized"}
	}
	return ompProbeResult{output: output}
}

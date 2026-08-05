package omp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type ompNativeRPCTrace struct {
	toolSequence []string
	hubOps       []string
	taskCalls    int
	pairedCalls  int
	starts       map[string]string
	ended        map[string]bool
}

func runActualOMPNativeRPC(
	ctx context.Context,
	executable, scratch, profile, overlay, prompt, allowedEndpoint string,
	provider *ompNativeProvider,
) (ompNativeRPCTrace, error) {
	trace := ompNativeRPCTrace{starts: make(map[string]string), ended: make(map[string]bool)}
	cmd := exec.CommandContext(ctx, executable,
		"--mode", "rpc",
		"--no-session",
		"--cwd", scratch,
		"--model", "s7dummy/"+ompLiveModel,
		"--config", overlay,
		"--tools", "task,hub",
		"--auto-approve",
		"--no-extensions",
		"--no-rules",
		"--no-lsp",
		"--no-pty",
		"--max-time", "40s",
	)
	cmd.Dir = scratch
	isolatedEnv, envErr := isolatedOMPLiveEnv(profile, overlay)
	if envErr != nil {
		return trace, fmt.Errorf("native_rpc_isolation_invalid")
	}
	cmd.Env = isolatedEnv
	cmd.Stderr = io.Discard
	cmd.WaitDelay = ompRPCWaitDelay
	if err := configureOMPRPCNetworkSandbox(cmd, allowedEndpoint); err != nil {
		return trace, fmt.Errorf("native_rpc_network_sandbox_unavailable: %w", err)
	}
	if err := configureOMPRPCProcessGroup(cmd); err != nil {
		return trace, fmt.Errorf("native_rpc_process_group_unsupported")
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return trace, fmt.Errorf("native_rpc_stdin_pipe")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return trace, fmt.Errorf("native_rpc_stdout_pipe")
	}
	if err := cmd.Start(); err != nil {
		return trace, fmt.Errorf("native_rpc_start")
	}
	waited := false
	completedCleanly := false
	defer func() {
		_ = stdin.Close()
		if !completedCleanly {
			provider.releaseAll()
			_ = terminateOMPRPCProcessGroup(cmd)
		}
		if !waited {
			_ = cmd.Wait()
		}
	}()

	frames, scanDone := scanOMPRPCFrames(stdout)
	ready := false
	for !ready {
		frame, readErr := nextOMPRPCFrame(ctx, frames, scanDone)
		if readErr != nil {
			return trace, readErr
		}
		ready = rpcFrameType(frame) == "ready"
	}
	encoder := json.NewEncoder(stdin)
	for _, command := range []map[string]any{
		{"id": "disable-retry", "type": "set_auto_retry", "enabled": false},
		{"id": "disable-compaction", "type": "set_auto_compaction", "enabled": false},
		{"id": "native-task-hub-prompt", "type": "prompt", "message": prompt},
	} {
		if encoder.Encode(command) != nil {
			return trace, fmt.Errorf("native_rpc_write_command")
		}
	}

	terminal := false
	for !terminal {
		frame, readErr := nextOMPRPCFrame(ctx, frames, scanDone)
		if readErr != nil {
			return trace, readErr
		}
		terminal, err = trace.consume(frame, provider)
		if err != nil {
			return trace, err
		}
	}
	if err := stdin.Close(); err != nil {
		return trace, fmt.Errorf("native_rpc_close_stdin")
	}
	waitErr := cmd.Wait()
	waited = true
	if waitErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return trace, fmt.Errorf("native_rpc_timeout")
		}
		return trace, fmt.Errorf("native_rpc_exit_nonzero")
	}
	completedCleanly = true
	return trace, nil
}

func (trace *ompNativeRPCTrace) consume(frame []byte, provider *ompNativeProvider) (bool, error) {
	var event struct {
		Type       string          `json:"type"`
		ToolCallID string          `json:"toolCallId"`
		ToolName   string          `json:"toolName"`
		Args       json.RawMessage `json:"args"`
		IsError    bool            `json:"isError"`
		Success    *bool           `json:"success"`
		Message    struct {
			Role       string `json:"role"`
			StopReason string `json:"stopReason"`
		} `json:"message"`
	}
	if json.Unmarshal(frame, &event) != nil {
		return false, nil
	}
	if event.Type == "response" && event.Success != nil && !*event.Success {
		return false, fmt.Errorf("native_rpc_response_failed")
	}
	switch event.Type {
	case "tool_execution_start":
		if event.ToolCallID == "" || event.ToolName == "" {
			return false, fmt.Errorf("native_rpc_tool_start_invalid")
		}
		trace.starts[event.ToolCallID] = event.ToolName
		switch event.ToolName {
		case "task":
			if trace.taskCalls != 0 {
				return false, fmt.Errorf("native_rpc_task_repeated")
			}
			var args any
			if err := json.Unmarshal(event.Args, &args); err != nil {
				return false, fmt.Errorf("native_rpc_task_arguments_decode")
			}
			if err := validateOMPNativeTaskArguments(args); err != nil {
				return false, fmt.Errorf("native_rpc_task_arguments_invalid:%w", err)
			}
			trace.taskCalls++
			trace.toolSequence = append(trace.toolSequence, "task")
		case "hub":
			var args struct {
				IDs       []string `json:"ids"`
				Op        string   `json:"op"`
				TimeoutMS int      `json:"timeoutMs"`
			}
			if json.Unmarshal(event.Args, &args) != nil || args.Op == "" {
				return false, fmt.Errorf("native_rpc_hub_arguments_invalid")
			}
			expected := []string{"list", "jobs", "wait", "jobs", "wait", "list"}
			if len(trace.hubOps) >= len(expected) || args.Op != expected[len(trace.hubOps)] {
				return false, fmt.Errorf("native_rpc_hub_sequence_invalid")
			}
			trace.hubOps = append(trace.hubOps, args.Op)
			trace.toolSequence = append(trace.toolSequence, "hub:"+args.Op)
			if args.Op == "wait" {
				expectedID := ompNativeAlphaID
				if len(trace.hubOps) == 5 {
					expectedID = ompNativeBetaID
				}
				if len(args.IDs) != 1 || args.IDs[0] != expectedID ||
					args.TimeoutMS < 1 || args.TimeoutMS > 15_000 {
					return false, fmt.Errorf("native_rpc_hub_wait_arguments_invalid")
				}
				provider.release(expectedID)
			}
		default:
			return false, fmt.Errorf("native_rpc_unexpected_tool:%s", event.ToolName)
		}
	case "tool_execution_end":
		name := trace.starts[event.ToolCallID]
		if name == "" || event.IsError {
			return false, fmt.Errorf("native_rpc_tool_end_invalid:%s", name)
		}
		if !trace.ended[event.ToolCallID] {
			trace.ended[event.ToolCallID] = true
			trace.pairedCalls++
		}
	case "message_end":
		if event.Message.Role == "assistant" && event.Message.StopReason == "stop" {
			want := []string{
				"task", "hub:list", "hub:jobs", "hub:wait", "hub:jobs", "hub:wait", "hub:list",
			}
			if strings.Join(trace.toolSequence, "\x00") != strings.Join(want, "\x00") ||
				trace.pairedCalls != 7 {
				return false, fmt.Errorf("native_rpc_terminal_before_lifecycle")
			}
			return true, nil
		}
	}
	return false, nil
}

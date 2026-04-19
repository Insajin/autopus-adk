package orchestra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// subprocessBackend implements ExecutionBackend by spawning provider CLIs
// as child processes with structured JSON I/O.
type subprocessBackend struct {
	schemaBuilder *SchemaBuilder
}

// NewSubprocessBackendImpl creates a fully functional SubprocessBackend.
func NewSubprocessBackendImpl() ExecutionBackend {
	return &subprocessBackend{
		schemaBuilder: &SchemaBuilder{},
	}
}

// Name returns "subprocess".
func (b *subprocessBackend) Name() string {
	return "subprocess"
}

// Execute spawns a provider CLI process, sends the prompt via stdin or args,
// and collects the JSON output from stdout.
func (b *subprocessBackend) Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	if req.Config.Binary == "" {
		return nil, fmt.Errorf("subprocess: provider %q has no binary configured", req.Provider)
	}

	// Apply per-provider timeout if specified
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	args := buildSubprocessArgs(req)
	start := time.Now()

	cmd := newCommand(ctx, req.Config.Binary, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.SetStdout(&stdoutBuf)
	cmd.SetStderr(&stderrBuf)

	if err := setupStdin(cmd, req); err != nil {
		return nil, err
	}

	waitErr := cmd.Wait()
	duration := time.Since(start)

	output := stdoutBuf.String()
	resp := &ProviderResponse{
		Provider:    req.Provider,
		Output:      output,
		Error:       stderrBuf.String(),
		Duration:    duration,
		ExitCode:    cmd.ExitCode(),
		EmptyOutput: strings.TrimSpace(output) == "",
	}

	if ctx.Err() != nil {
		resp.TimedOut = true
		return resp, nil
	}

	if waitErr != nil && resp.ExitCode != 0 {
		return resp, fmt.Errorf("subprocess %s exited with code %d: %w",
			req.Provider, resp.ExitCode, waitErr)
	}

	// Validate JSON output when a role is specified (structured output expected)
	if req.Role != "" && !resp.EmptyOutput {
		if err := validateJSONOutput(output); err != nil {
			resp.Error = fmt.Sprintf("invalid JSON output: %v", err)
		}
	}

	return resp, nil
}

// buildSubprocessArgs constructs command arguments for subprocess execution.
func buildSubprocessArgs(req ProviderRequest) []string {
	args := append([]string{}, req.Config.Args...)

	// Append schema path if available
	if req.SchemaPath != "" {
		args = append(args, "--schema", req.SchemaPath)
	}

	// Pass prompt via args when configured (e.g., gemini -p "prompt")
	if req.Config.PromptViaArgs {
		args = append(args, "-p", req.Prompt)
	}

	return args
}

// setupStdin configures stdin for the command. For PromptViaArgs providers,
// stdin is closed. Otherwise, the prompt is piped via stdin.
// If the prompt exceeds maxStdinLen, it is written to a temp file and the
// path is appended to args via --prompt-file.
func setupStdin(cmd command, req ProviderRequest) error {
	if req.Config.PromptViaArgs {
		// Close stdin explicitly to prevent provider from waiting for input.
		// nil means inherit parent stdin which can hang subprocess providers.
		devNull, err := os.Open(os.DevNull)
		if err != nil {
			return fmt.Errorf("subprocess %s: open devnull: %w", req.Provider, err)
		}
		cmd.SetStdin(devNull)
		return cmd.Start()
	}

	// For large prompts, use file-based input
	if len(req.Prompt) > maxStdinLen {
		return startWithFileInput(cmd, req)
	}

	return startWithStdinPipe(cmd, req)
}

// maxStdinLen is the threshold above which prompts are written to a temp file.
const maxStdinLen = 64 * 1024

// startWithStdinPipe sends the prompt via stdin pipe.
func startWithStdinPipe(cmd command, req ProviderRequest) error {
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("subprocess %s: stdin pipe: %w", req.Provider, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("subprocess %s: start: %w", req.Provider, err)
	}
	if _, err := io.WriteString(stdinPipe, req.Prompt); err != nil {
		_ = cmd.Wait()
		return fmt.Errorf("subprocess %s: write stdin: %w", req.Provider, err)
	}
	_ = stdinPipe.Close()
	return nil
}

// startWithFileInput writes the prompt to a temp file for large inputs.
func startWithFileInput(cmd command, req ProviderRequest) error {
	f, err := os.CreateTemp("", "prompt-*.txt")
	if err != nil {
		return fmt.Errorf("subprocess %s: create temp prompt file: %w", req.Provider, err)
	}
	defer removePromptFile(f.Name())

	if _, err := f.WriteString(req.Prompt); err != nil {
		_ = f.Close()
		return fmt.Errorf("subprocess %s: write prompt file: %w", req.Provider, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("subprocess %s: close prompt file: %w", req.Provider, err)
	}

	// Reopen as reader for stdin
	reader, err := os.Open(f.Name())
	if err != nil {
		return fmt.Errorf("subprocess %s: reopen prompt file: %w", req.Provider, err)
	}
	defer reader.Close()

	cmd.SetStdin(reader)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("subprocess %s: start: %w", req.Provider, err)
	}
	return nil
}

func removePromptFile(path string) {
	_ = os.Remove(path)
}

// validateJSONOutput checks that the output is valid JSON.
func validateJSONOutput(output string) error {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return fmt.Errorf("empty output")
	}
	if !json.Valid([]byte(trimmed)) {
		// Try to extract JSON from output that may contain leading/trailing text
		start := strings.IndexByte(trimmed, '{')
		if start < 0 {
			return fmt.Errorf("no JSON object found in output")
		}
		end := strings.LastIndexByte(trimmed, '}')
		if end <= start {
			return fmt.Errorf("malformed JSON object in output")
		}
		if !json.Valid([]byte(trimmed[start : end+1])) {
			return fmt.Errorf("invalid JSON in output")
		}
	}
	return nil
}

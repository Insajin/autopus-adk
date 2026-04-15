package orchestra

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// providerResult holds the result of a single provider execution.
type providerResult struct {
	resp ProviderResponse
	err  error
	idx  int
}

// runProvider executes a single provider and returns its response.
// @AX:ANCHOR: [AUTO] internal fan_in=6; signature is a stable contract
func runProvider(ctx context.Context, provider ProviderConfig, prompt string) (*ProviderResponse, error) {
	start := time.Now()

	args := append([]string{}, provider.Args...)
	if provider.PromptViaArgs {
		args = append(args, "-p", prompt)
	}

	cmd := newCommand(ctx, provider.Binary, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.SetStdout(&stdoutBuf)
	cmd.SetStderr(&stderrBuf)

	if !provider.PromptViaArgs {
		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("%s stdin 파이프 생성 실패: %w", provider.Name, err)
		}

		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("%s 시작 실패: %w", provider.Name, err)
		}
		waitCh := startCommandWait(cmd)

		if _, err := io.WriteString(stdinPipe, prompt); err != nil {
			_ = cmd.Terminate(provider.Name + " stdin write failure")
			drainCommandWait(waitCh)
			return nil, fmt.Errorf("%s stdin 쓰기 실패: %w", provider.Name, err)
		}
		_ = stdinPipe.Close()

		waitErr := waitForCommand(ctx, cmd, provider.Name, waitCh)
		return buildProviderResponse(start, provider, &stdoutBuf, &stderrBuf, waitErr, ctx, cmd.ExitCode())
	}

	cmd.SetStdin(nil)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s 시작 실패: %w", provider.Name, err)
	}

	waitCh := startCommandWait(cmd)
	waitErr := waitForCommand(ctx, cmd, provider.Name, waitCh)
	return buildProviderResponse(start, provider, &stdoutBuf, &stderrBuf, waitErr, ctx, cmd.ExitCode())
}

func startCommandWait(cmd command) <-chan error {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()
	return waitCh
}

func waitForCommand(ctx context.Context, cmd command, providerName string, waitCh <-chan error) error {
	select {
	case err := <-waitCh:
		return err
	default:
	}

	select {
	case err := <-waitCh:
		return err
	case <-ctx.Done():
		_ = cmd.Terminate(providerName + " context cancelled")
		select {
		case err := <-waitCh:
			return err
		case <-time.After(providerWaitGracePeriod):
			return ctx.Err()
		}
	}
}

func drainCommandWait(waitCh <-chan error) {
	select {
	case <-waitCh:
	case <-time.After(providerWaitGracePeriod):
	}
}

func buildProviderResponse(start time.Time, provider ProviderConfig, stdoutBuf, stderrBuf *bytes.Buffer, waitErr error, ctx context.Context, exitCode int) (*ProviderResponse, error) {
	duration := time.Since(start)

	output := stdoutBuf.String()
	resp := &ProviderResponse{
		Provider:    provider.Name,
		Output:      output,
		Error:       stderrBuf.String(),
		Duration:    duration,
		ExitCode:    exitCode,
		EmptyOutput: strings.TrimSpace(output) == "",
	}

	if ctx.Err() != nil {
		resp.TimedOut = true
	}

	if waitErr != nil && !resp.TimedOut && resp.ExitCode != 0 {
		return resp, fmt.Errorf("%s 실행 오류 (exit %d): %w", provider.Name, resp.ExitCode, waitErr)
	}

	return resp, nil
}

func buildAllProvidersFailedError(failed []FailedProvider, fallback error) error {
	if fallback == nil && allFailuresTimedOut(failed) {
		return nil
	}

	if len(failed) == 0 {
		if fallback != nil {
			return fallback
		}
		return fmt.Errorf("모든 프로바이더가 실패했습니다")
	}

	details := make([]string, 0, len(failed))
	for _, fp := range failed {
		details = append(details, fmt.Sprintf("%s: %s", fp.Name, fp.Error))
	}

	if fallback != nil {
		return fmt.Errorf("모든 프로바이더가 실패했습니다 (%s): %w", strings.Join(details, "; "), fallback)
	}
	return fmt.Errorf("모든 프로바이더가 실패했습니다: %s", strings.Join(details, "; "))
}

func allFailuresTimedOut(failed []FailedProvider) bool {
	if len(failed) == 0 {
		return false
	}
	for _, fp := range failed {
		if !strings.HasPrefix(fp.Error, "timeout:") {
			return false
		}
	}
	return true
}

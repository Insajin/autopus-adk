package codexruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	catalogProbeStartAttempts   = 5
	catalogProbeStartRetryDelay = 10 * time.Millisecond
)

// ProbeModelCatalog reads and validates a bounded `codex debug models` response.
func ProbeModelCatalog(ctx context.Context, binary string, timeout time.Duration) ([]byte, error) {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for attempt := 0; ; attempt++ {
		output, err := probeModelCatalogOnce(probeCtx, binary)
		if err == nil || !errors.Is(err, syscall.ETXTBSY) ||
			attempt+1 >= catalogProbeStartAttempts {
			return output, err
		}
		select {
		case <-probeCtx.Done():
			return nil, fmt.Errorf("codex model catalog probe timed out: %w", probeCtx.Err())
		case <-time.After(catalogProbeStartRetryDelay):
		}
	}
}

func probeModelCatalogOnce(probeCtx context.Context, binary string) ([]byte, error) {
	cmd := exec.CommandContext(probeCtx, binary, "debug", "models")
	cmd.WaitDelay = time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open codex model catalog stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex model catalog probe: %w", err)
	}

	type readResult struct {
		output []byte
		err    error
	}
	readDone := make(chan readResult, 1)
	go func() {
		output, readErr := io.ReadAll(io.LimitReader(stdout, config.MaxCodexModelCatalogBytes+1))
		readDone <- readResult{output: output, err: readErr}
	}()

	var result readResult
	select {
	case result = <-readDone:
	case <-probeCtx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("codex model catalog probe timed out: %w", probeCtx.Err())
	}
	output, readErr := result.output, result.err
	if len(output) > config.MaxCodexModelCatalogBytes {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("codex model catalog exceeds %d bytes", config.MaxCodexModelCatalogBytes)
	}
	if readErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("read codex model catalog: %w", readErr)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("run codex model catalog probe: %w", err)
	}
	if err := config.ValidateCodexModelCatalogPayload(output); err != nil {
		return nil, err
	}
	return output, nil
}

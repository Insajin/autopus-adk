package omp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPModelProbeProcess_RunUsesPinnedCredentialFreeSandbox(t *testing.T) {
	secret := "OMP-MODEL-PROBE-SECRET-SENTINEL"
	t.Setenv("OMP_MODEL_PROBE_SECRET", secret)
	t.Setenv("OPENAI_API_KEY", secret)

	executable := writeOMPModelProbeExecutable(t, `#!/bin/sh
printf 'args=%s\n' "$*"
printf 'home=%s\n' "$HOME"
printf 'tmp=%s\n' "$TMPDIR"
printf 'config=%s\n' "$XDG_CONFIG_HOME"
printf 'data=%s\n' "$XDG_DATA_HOME"
printf 'cache=%s\n' "$XDG_CACHE_HOME"
printf 'lang=%s/%s\n' "$LANG" "$LC_ALL"
printf 'secret=%s/%s\n' "${OMP_MODEL_PROBE_SECRET-unset}" "${OPENAI_API_KEY-unset}"
printf 'cwd=%s\n' "$PWD"
`)
	process, err := NewOMPModelProbeProcess(executable, 4096)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := process.Run(ctx, "models", "--json")
	require.NoError(t, err)
	text := string(output)
	assert.Contains(t, text, "args=models --json\n")
	assert.Contains(t, text, "lang=C/C\n")
	assert.Contains(t, text, "secret=unset/unset\n")
	assert.NotContains(t, text, secret)
	for _, name := range []string{"home", "tmp", "config", "data", "cache"} {
		line := ompModelProbeOutputValue(t, text, name)
		assert.Contains(t, line, "autopus-omp-model-probe-")
		assert.True(t, filepath.IsAbs(line))
	}
	assert.Contains(t, ompModelProbeOutputValue(t, text, "cwd"), "autopus-omp-model-probe-")
}

func TestOMPModelProbeProcess_RunEnforcesOutputLimit(t *testing.T) {
	t.Parallel()

	executable := writeOMPModelProbeExecutable(t, "#!/bin/sh\nwhile :; do printf '0123456789'; done\n")
	process, err := NewOMPModelProbeProcess(executable, 64)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := time.Now()
	output, err := process.Run(ctx, "models", "--json")
	assert.ErrorIs(t, err, processprobe.ErrOutputLimit)
	assert.Less(t, time.Since(started), 2*time.Second)
	assert.LessOrEqual(t, len(output), 64)
}

func TestOMPModelProbeProcess_RunHonorsContextTimeout(t *testing.T) {
	root := t.TempDir()
	startedFile := filepath.Join(root, "started")
	executable := writeOMPModelProbeExecutable(t, "#!/bin/sh\nprintf 'started\\n'\nprintf ready > '"+
		strings.ReplaceAll(startedFile, "'", "'\"'\"'")+"'\nsleep 10\n")
	process, err := NewOMPModelProbeProcess(executable, 1024)
	require.NoError(t, err)

	ctx := newOMPModelProbeDeadlineContext()
	defer func() {
		if ctx.Err() == nil {
			ctx.expire()
		}
	}()
	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, runErr := process.Run(ctx, "--version")
		done <- result{output: output, err: runErr}
	}()
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(startedFile)
		return statErr == nil
	}, 10*time.Second, 10*time.Millisecond)

	started := time.Now()
	ctx.expire()
	run := <-done
	err = run.err
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded))
	assert.Less(t, time.Since(started), 5*time.Second)
	assert.Equal(t, []byte("started\n"), run.output)
}

type ompModelProbeDeadlineContext struct {
	context.Context
	done chan struct{}
}

func newOMPModelProbeDeadlineContext() *ompModelProbeDeadlineContext {
	return &ompModelProbeDeadlineContext{Context: context.Background(), done: make(chan struct{})}
}

func (ctx *ompModelProbeDeadlineContext) Done() <-chan struct{} {
	return ctx.done
}

func (ctx *ompModelProbeDeadlineContext) Err() error {
	select {
	case <-ctx.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (ctx *ompModelProbeDeadlineContext) expire() {
	close(ctx.done)
}

func TestOMPModelProbeProcess_RunRejectsChangedOrUnpinnedIdentity(t *testing.T) {
	t.Parallel()

	var nilProcess *OMPModelProbeProcess
	output, err := nilProcess.Run(context.Background(), "--version")
	assert.Nil(t, output)
	assert.ErrorContains(t, err, "not pinned")

	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	executable := filepath.Join(dir, "omp-fixture")
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\nprintf old > "+marker+"\n"), 0o700))
	process, err := NewOMPModelProbeProcess(executable, 1024)
	require.NoError(t, err)
	require.NoError(t, os.Rename(executable, executable+".old"))
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\nprintf changed > "+marker+"\n"), 0o700))

	output, err = process.Run(context.Background(), "--version")
	assert.Nil(t, output)
	assert.ErrorContains(t, err, "identity changed")
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestNewOMPModelProbeProcess_RejectsInvalidExecutableContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		executable string
		maxOutput  int
		want       string
	}{
		{name: "empty executable", maxOutput: 1, want: "inputs are invalid"},
		{name: "zero output budget", executable: "/bin/sh", want: "inputs are invalid"},
		{name: "missing executable", executable: filepath.Join(t.TempDir(), "missing"), maxOutput: 1, want: "resolve"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			process, err := NewOMPModelProbeProcess(tc.executable, tc.maxOutput)
			assert.Nil(t, process)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func writeOMPModelProbeExecutable(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "omp-model-probe-fixture")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o700))
	return path
}

func ompModelProbeOutputValue(t *testing.T, output, key string) string {
	t.Helper()
	prefix := key + "="
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	t.Fatalf("missing %s output in %q", key, output)
	return ""
}

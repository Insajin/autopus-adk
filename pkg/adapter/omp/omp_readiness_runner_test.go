package omp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

func TestOMPReadinessRunner_UsesSanitizedSandboxAndRequestedCWD(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	workspace := t.TempDir()
	sandbox, err := createOMPReadinessSandbox()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sandbox) })
	executable := filepath.Join(t.TempDir(), "omp-runner-fixture")
	script := `#!/bin/sh
printf 'cwd=%s\n' "$PWD"
printf 'home=%s\n' "$HOME"
printf 'profile=%s\n' "$PI_CODING_AGENT_DIR"
printf 'credential=%s\n' "${OPENAI_API_KEY-unset}"
printf 'args=%s\n' "$*"
`
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o755))
	t.Setenv("OPENAI_API_KEY", "must-not-cross")

	runner, resolved := configureOMPProbeRunner(
		commandOMPProbeRunner{maxOutput: 4096}, executable, sandbox, workspace,
	)
	output, err := runner.Run(t.Context(), executable, "--version")
	require.NoError(t, err)
	normalizePath := func(value string) string {
		return strings.ReplaceAll(value, "/private/var/", "/var/")
	}
	assert.Equal(t, normalizePath(executable), normalizePath(resolved))
	text := normalizePath(string(output))
	assert.Contains(t, text, "cwd="+normalizePath(workspace)+"\n")
	assert.Contains(t, text, "home="+normalizePath(filepath.Join(sandbox, "home"))+"\n")
	assert.Contains(t, text, "profile="+normalizePath(filepath.Join(sandbox, "pi-agent"))+"\n")
	assert.Contains(t, text, "credential=unset\n")
	assert.False(t, strings.Contains(text, "must-not-cross"))
}

func TestOMPReadinessRunner_FailsClosedOnMissingBinaryAndOversizedOutput(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	t.Run("missing binary", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := (commandOMPProbeRunner{maxOutput: 64}).Run(context.Background(), "not-omp")
		require.Error(t, err)
		assert.False(t, errors.Is(err, processprobe.ErrOutputLimit))
	})

	t.Run("bounded output", func(t *testing.T) {
		executable := filepath.Join(t.TempDir(), "omp-output-fixture")
		require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\nprintf '0123456789abcdef'\n"), 0o755))
		_, err := (commandOMPProbeRunner{maxOutput: 8}).Run(context.Background(), executable)
		require.Error(t, err)
		assert.True(t, errors.Is(err, processprobe.ErrOutputLimit))
	})
}

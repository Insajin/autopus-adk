package omp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

func TestOMPReadinessRunner_UsesConfigOwnedProfileAndBoundedOutput(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	binDir := t.TempDir()
	profile := t.TempDir()
	executable := filepath.Join(binDir, "omp-runner-fixture")
	script := `#!/bin/sh
printf 'profile=%s\n' "$PI_CODING_AGENT_DIR"
printf 'args=%s\n' "$*"
`
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o755))
	t.Setenv("PATH", binDir)

	runner := commandOMPProbeRunner{maxOutput: 4096}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := runner.Run(ctx, filepath.Base(executable),
		"--config", filepath.Join(profile, "config.yml"), "models", "--json")
	require.NoError(t, err)
	assert.Equal(t,
		"profile="+profile+"\nargs=--config "+filepath.Join(profile, "config.yml")+" models --json\n",
		string(output),
	)
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
		binDir := t.TempDir()
		executable := filepath.Join(binDir, "omp-output-fixture")
		require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\nprintf '0123456789abcdef'\n"), 0o755))
		t.Setenv("PATH", binDir)

		_, err := (commandOMPProbeRunner{maxOutput: 8}).Run(context.Background(), filepath.Base(executable))
		require.Error(t, err)
		assert.True(t, errors.Is(err, processprobe.ErrOutputLimit))
	})
}

func TestOMPProbeProfileDir_RequiresPairedConfigValue(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "none", args: []string{"models", "--json"}},
		{name: "dangling", args: []string{"models", "--config"}},
		{name: "paired", args: []string{"--config", "/fixture/profile/config.yml", "models"}, want: "/fixture/profile"},
		{name: "first overlay owns profile", args: []string{"--config", "/first/config.yml", "--config", "/second/config.yml"}, want: "/first"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ompProbeProfileDir(tc.args))
		})
	}
}

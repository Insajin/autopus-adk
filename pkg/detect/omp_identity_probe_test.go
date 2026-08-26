package detect

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeOMPIdentity_RequiresExactReleaseIdentity(t *testing.T) {
	skipOMPIdentityProbeWithoutShell(t)

	for _, test := range []struct {
		name, output string
		want         bool
	}{
		{name: "release", output: "omp/18.0.5", want: true},
		{name: "prerelease", output: "omp/18.0.5-rc.1"},
		{name: "extra output", output: "omp/18.0.5\\nextra"},
		{name: "impostor", output: "omp 18.0.5"},
	} {
		t.Run(test.name, func(t *testing.T) {
			binary := writeOMPIdentityProbe(t, "printf "+strconv.Quote(test.output+"\n")+"\n")
			version, ok := ProbeOMPIdentity(context.Background(), binary)
			assert.Equal(t, test.want, ok)
			if test.want {
				assert.Equal(t, test.output, version)
			} else {
				assert.Empty(t, version)
			}
		})
	}
}

func TestProbeOMPIdentity_UsesPrivateCredentialFreeSandbox(t *testing.T) {
	skipOMPIdentityProbeWithoutShell(t)

	const sentinel = "OMP_IDENTITY_SECRET_MUST_NOT_LEAK"
	for _, key := range []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY",
		"OMP_ACCESS_TOKEN", "PI_PROVIDER_TOKEN", "PI_CODING_AGENT_DIR",
	} {
		t.Setenv(key, sentinel)
	}
	t.Setenv("HOME", sentinel)
	marker := filepath.Join(t.TempDir(), "observed")
	script := "{\n" +
		"printf 'cwd=%s\\n' \"$PWD\"\n" +
		"printf 'home=%s\\n' \"$HOME\"\n" +
		"printf 'profile=%s\\n' \"${PI_CODING_AGENT_DIR-unset}\"\n" +
		"printf 'credentials=%s/%s/%s/%s/%s\\n' \"${OPENAI_API_KEY-unset}\" \"${ANTHROPIC_API_KEY-unset}\" \"${GOOGLE_API_KEY-unset}\" \"${OMP_ACCESS_TOKEN-unset}\" \"${PI_PROVIDER_TOKEN-unset}\"\n" +
		"} > " + strconv.Quote(marker) + "\n" +
		"printf 'omp/18.0.5\\n'\n"

	version, ok := ProbeOMPIdentity(context.Background(), writeOMPIdentityProbe(t, script))
	require.True(t, ok)
	assert.Equal(t, "omp/18.0.5", version)
	observed, err := os.ReadFile(marker)
	require.NoError(t, err)
	text := string(observed)
	assert.NotContains(t, text, sentinel)
	assert.Contains(t, text, "profile=unset\n")
	assert.Contains(t, text, "credentials=unset/unset/unset/unset/unset\n")
	lines := strings.Split(strings.TrimSpace(text), "\n")
	require.GreaterOrEqual(t, len(lines), 2)
	cwd := strings.TrimPrefix(lines[0], "cwd=")
	home := strings.TrimPrefix(lines[1], "home=")
	assert.Contains(t, filepath.Base(cwd), "autopus-omp-identity-")
	normalizedCWD := filepath.Clean(strings.TrimPrefix(cwd, "/private"))
	normalizedHome := filepath.Clean(strings.TrimPrefix(home, "/private"))
	assert.Equal(t, filepath.Join(normalizedCWD, "home"), normalizedHome)
}

func TestProbeOMPIdentity_BoundsTimeoutAndCombinedOutput(t *testing.T) {
	skipOMPIdentityProbeWithoutShell(t)

	t.Run("parent deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		version, ok := ProbeOMPIdentity(ctx, writeOMPIdentityProbe(t, "sleep 5\nprintf 'omp/18.0.5\\n'\n"))
		assert.False(t, ok)
		assert.Empty(t, version)
	})

	t.Run("output budget", func(t *testing.T) {
		script := "i=0\nwhile [ \"$i\" -lt 70000 ]; do printf x; i=$((i + 1)); done\nprintf 'omp/18.0.5\\n'\n"
		version, ok := ProbeOMPIdentity(context.Background(), writeOMPIdentityProbe(t, script))
		assert.False(t, ok)
		assert.Empty(t, version)
	})
}

func writeOMPIdentityProbe(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "omp")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755))
	return path
}

func skipOMPIdentityProbeWithoutShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}
}

//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package gemini

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestValidatePluginListInheritedPipeDegradesWithinBound(t *testing.T) {
	a := generatedGeminiAdapterForProbe(t)
	installAgyProbe(t, "#!/bin/sh\n(/bin/sleep 5) &\nprintf '{\"plugins\":[]}\\n'\n")
	started := time.Now()

	errs, err := a.Validate(context.Background())

	require.NoError(t, err)
	assert.Empty(t, errs, "a failed best-effort plugin probe must not create a validation finding")
	assert.Less(t, time.Since(started), 2*time.Second,
		"validation must not wait for a grandchild that inherited the probe pipes")
}

func TestValidatePluginListHonorsCallerContext(t *testing.T) {
	a := generatedGeminiAdapterForProbe(t)
	installAgyProbe(t, "#!/bin/sh\n/bin/sleep 5\n")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()

	errs, err := a.Validate(ctx)

	require.NoError(t, err)
	assert.Empty(t, errs)
	assert.Less(t, time.Since(started), time.Second,
		"the caller deadline must stop the plugin probe before its child timeout")
}

func TestValidatePluginListPreservesSuccessfulProbeSemantics(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantWarning bool
	}{
		{name: "autopus installed", output: `{"plugins":[{"name":"autopus"}]}`, wantWarning: false},
		{name: "autopus missing", output: `{"plugins":[]}`, wantWarning: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := generatedGeminiAdapterForProbe(t)
			installAgyProbe(t, "#!/bin/sh\nprintf '%s\\n' '"+tt.output+"'\n")

			errs, err := a.Validate(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.wantWarning, containsAutopusPluginWarning(errs))
		})
	}
}

func generatedGeminiAdapterForProbe(t *testing.T) *Adapter {
	t.Helper()
	a := NewWithRoot(t.TempDir())
	_, err := a.Generate(context.Background(), config.DefaultFullConfig("probe-test"))
	require.NoError(t, err)
	return a
}

func installAgyProbe(t *testing.T, content string) {
	t.Helper()
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, cliBinary), []byte(content), 0o755))
	t.Setenv("PATH", binDir)
}

func containsAutopusPluginWarning(errs []adapter.ValidationError) bool {
	for _, validationErr := range errs {
		if validationErr.File == ".agents/plugins/autopus" && validationErr.Level == "warning" {
			return true
		}
	}
	return false
}

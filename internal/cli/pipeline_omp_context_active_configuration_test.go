package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func TestPipelineOMPMaxTimeSeconds_UsesPositiveWholeSeconds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		maxTime time.Duration
		want    string
	}{
		{name: "ten minutes", maxTime: 10 * time.Minute, want: "600"},
		{name: "subsecond", maxTime: time.Nanosecond, want: "1"},
		{name: "fractional second", maxTime: 1500 * time.Millisecond, want: "2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, pipelineOMPMaxTimeSeconds(test.maxTime))
		})
	}
}

func TestWritePipelineOMPActiveModels_BindsConfiguredContextWindow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	active := pipelineOMPActiveProcessConfig{
		backend: pipelineOMPBackendConfig{
			PhaseModels:        map[pipeline.PhaseID]string{pipeline.PhaseImplement: "openai/model-a"},
			ModelContextWindow: 1_000_000,
		},
		endpoint: "http://127.0.0.1:43123",
	}

	require.NoError(t, writePipelineOMPActiveModels(root, active))
	body, err := os.ReadFile(filepath.Join(root, "models.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "contextWindow: 1000000")
	assert.NotContains(t, string(body), "contextWindow: 262144")
	assert.NotContains(t, string(body), "input:")
}

func TestConfigurePipelineOMPActiveSandbox_InheritedParentSkipsInnerWrapperOnDarwin(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("/usr/bin/true")
	originalPath := cmd.Path
	err := configurePipelineOMPActiveSandbox(
		cmd, "http://127.0.0.1:43123", pipelineOMPActiveSandboxInheritedParent,
	)
	if runtime.GOOS != "darwin" {
		require.ErrorContains(t, err, "Darwin")
		return
	}
	require.NoError(t, err)
	assert.Equal(t, originalPath, cmd.Path)
	assert.Equal(t, []string{"/usr/bin/true"}, cmd.Args)
}

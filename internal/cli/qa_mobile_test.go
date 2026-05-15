package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQAMobileReadinessPlanJSONIsSideEffectFree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n"), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"plan", "--project-dir", dir, "--lane", "mobile-readiness", "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	readiness := data["mobile_readiness"].(map[string]any)
	assert.Equal(t, "setup_gap", readiness["status"])
	assert.Empty(t, data["selected_adapters"])
	assert.Empty(t, data["selected_journeys"])
	assert.Empty(t, data["manifest_output_preview_paths"])
	assert.NoDirExists(t, filepath.Join(dir, ".autopus", "qa", "runs"))
}

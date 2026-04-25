package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_QualityUltraSetsCodexReasoningEffort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "codex", "--yes", "--quality", "ultra"})
	err := cmd.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	require.NoError(t, err)
	rootSection := strings.SplitN(string(data), "[agents]", 2)[0]
	assert.Contains(t, rootSection, `model_reasoning_effort = "xhigh"`)
}

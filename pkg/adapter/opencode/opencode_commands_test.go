package opencode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestAdapter_Generate_WorkflowCommandsRouteAliasesThroughAuto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	autoGo, err := os.ReadFile(filepath.Join(dir, ".opencode", "commands", "auto-go.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoGo), "`/auto go ...` payload로 다시 해석")
	assert.Contains(t, string(autoGo), "`skill` 도구로 `auto`를 로드")
	assert.NotContains(t, string(autoGo), "`skill` 도구로 `auto-go`를 로드")

	autoStatus, err := os.ReadFile(filepath.Join(dir, ".opencode", "commands", "auto-status.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoStatus), "`/auto status ...` payload로 다시 해석")
	assert.Contains(t, string(autoStatus), "`skill` 도구로 `auto`를 로드")
	assert.NotContains(t, string(autoStatus), "`skill` 도구로 `auto-status`를 로드")

	autoQA, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-qa", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoQA), "QAMESH")
	assert.Contains(t, string(autoQA), "auto qa plan")
	assert.Contains(t, string(autoQA), "auto qa run")
	assert.Contains(t, string(autoQA), "auto qa evidence")
	assert.Contains(t, string(autoQA), "auto qa feedback")
}

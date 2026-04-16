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

func TestAdapter_Generate_AutoSyncCarriesCompletionGates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	autoSyncSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-sync", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoSyncSkill), "## Completion Gates")
	assert.Contains(t, string(autoSyncSkill), "@AX: no-op")
	assert.Contains(t, string(autoSyncSkill), "commit hash")
	assert.Contains(t, string(autoSyncSkill), "현재 OpenCode 런타임 정책")
	assert.NotContains(t, string(autoSyncSkill), "현재 Codex 런타임 정책")
}

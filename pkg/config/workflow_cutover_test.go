package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowConfig_LegacyTeamDefaultHasNoRuntimeOrPersistedEffect(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	legacy := "project_name: cutover\nmode: full\nplatforms:\n  - claude-code\nworkflow:\n  team_default: false\n  coverage_threshold: 91\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, configFileName), []byte(legacy), 0o644))

	cfg, err := LoadPreview(root)
	require.NoError(t, err)
	assert.Equal(t, 91, cfg.Workflow.CoverageThreshold)
	require.NoError(t, Save(root, cfg))
	persisted, err := os.ReadFile(filepath.Join(root, configFileName))
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(persisted), "team_default"))
}

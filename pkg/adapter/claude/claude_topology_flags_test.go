package claude_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestAutoGoTopologyFlags_FailClosedBeforeRouteSelection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := claude.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("topology"))
	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "auto-go", "SKILL.md"))
	require.NoError(t, err)
	contract := string(body)

	gate := strings.Index(contract, "TOPOLOGY_FLAG_COUNT")
	selection := strings.Index(contract, "Determine execution mode")
	require.NotEqual(t, -1, gate)
	require.NotEqual(t, -1, selection)
	assert.Less(t, gate, selection)
	assert.Contains(t, contract, "TEAMS_MODE + WORKFLOW_MODE + SOLO_MODE")
	assert.Contains(t, contract, "TOPOLOGY_FLAG_COUNT > 1")
	assert.Contains(t, contract, "[TOPOLOGY ERROR] --team, --workflow, and --solo are mutually exclusive; choose only one.")
}

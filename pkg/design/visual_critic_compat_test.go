package design

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadVisualCriticReport_SymlinkWorkspaceRoot_UsesResolvedTarget(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "workspace")
	require.NoError(t, os.Mkdir(target, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(target, "critic.json"),
		[]byte(`{"status":"PASS"}`),
		0o600,
	))
	root := filepath.Join(parent, "workspace-link")
	if err := os.Symlink(target, root); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	report, err := LoadVisualCriticReport(root, "critic.json")
	require.NoError(t, err)
	assert.Equal(t, "PASS", report.Status)
	assert.Equal(t, "critic.json", report.Source)
}

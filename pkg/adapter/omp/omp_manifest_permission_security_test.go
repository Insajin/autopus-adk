package omp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPClean_RejectsWorldReadableManifestClaimingUserSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillPath := filepath.Join(".omp", "skills", "user-skill", "SKILL.md")
	skillContent := []byte("user-owned skill\n")
	fullSkillPath := filepath.Join(root, skillPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullSkillPath), 0o755))
	require.NoError(t, os.WriteFile(fullSkillPath, skillContent, 0o644))

	manifest := adapter.NewManifest(adapterName)
	manifest.Files[skillPath] = adapter.ManifestFile{
		Checksum: adapter.Checksum(string(skillContent)),
		Policy:   adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))
	manifestPath := filepath.Join(root, ".autopus", adapterName+"-manifest.json")
	manifestInfo, err := os.Stat(manifestPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), manifestInfo.Mode().Perm())

	err = NewWithRoot(root).Clean(context.Background())
	require.ErrorContains(t, err, "must have mode 0600")
	preserved, readErr := os.ReadFile(fullSkillPath)
	require.NoError(t, readErr)
	assert.Equal(t, skillContent, preserved)
	assert.FileExists(t, manifestPath)
}

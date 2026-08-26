package cli

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveConfigWithGenerationRollback_UnwindsCommittedPlatforms(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	original := config.DefaultFullConfig("before")
	original.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(root, original))
	_, err := claude.NewWithRoot(root).Generate(context.Background(), original)
	require.NoError(t, err)
	configBefore := readLifecycleBytes(t, filepath.Join(root, "autopus.yaml"))
	claudeBefore := readLifecycleBytes(t, filepath.Join(root, "CLAUDE.md"))
	manifestBefore := readLifecycleBytes(t, filepath.Join(root, ".autopus", "claude-code-manifest.json"))

	candidate := config.DefaultFullConfig("after")
	candidate.Platforms = []string{"claude-code", "codex"}
	generationErr := errors.New("later platform failed")
	err = saveConfigWithGenerationRollback(root, candidate, func() error {
		_, generateErr := claude.NewWithRoot(root).Generate(context.Background(), candidate)
		if generateErr != nil {
			return generateErr
		}
		_, generateErr = codex.NewWithRoot(root).Generate(context.Background(), candidate)
		if generateErr != nil {
			return generateErr
		}
		return generationErr
	})

	require.ErrorIs(t, err, generationErr)
	assert.Equal(t, configBefore, readLifecycleBytes(t, filepath.Join(root, "autopus.yaml")))
	assert.Equal(t, claudeBefore, readLifecycleBytes(t, filepath.Join(root, "CLAUDE.md")))
	assert.Equal(t, manifestBefore, readLifecycleBytes(t, filepath.Join(root, ".autopus", "claude-code-manifest.json")))
	assert.NoFileExists(t, filepath.Join(root, ".autopus", "codex-manifest.json"))
	assert.NoDirExists(t, filepath.Join(root, ".codex", "skills"))
}

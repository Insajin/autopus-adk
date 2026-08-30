package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestApplyOMPProfileRejectsRolePolicyPlaceholderBeforePreview(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	configPath, runner := writeOMPApplySecurityFixture(t, root)
	original, err := os.ReadFile(configPath)
	require.NoError(t, err)
	original = append(original, []byte("role_model_policy:\n  profile: ${OMP_PROFILE}\n")...)
	require.NoError(t, os.WriteFile(configPath, original, 0o640))
	t.Setenv("OMP_PROFILE", "balanced")
	activated := false

	_, err = applyOMPProfile(
		context.Background(), root, "balanced", runner,
		func(context.Context, string, *config.HarnessConfig) error {
			activated = true
			return nil
		},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "role_model_policy_placeholder_unsupported")
	assert.Empty(t, runner.calls)
	assert.False(t, activated)
	after, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, after)
}

func TestApplyOMPProfileRejectsSymlinkWorkspaceAndConfig(t *testing.T) {
	t.Run("workspace", func(t *testing.T) {
		parent := t.TempDir()
		realRoot := filepath.Join(parent, "real")
		configPath, runner := writeOMPApplySecurityFixture(t, realRoot)
		before, err := os.ReadFile(configPath)
		require.NoError(t, err)
		linkedRoot := filepath.Join(parent, "linked")
		if err := os.Symlink(realRoot, linkedRoot); err != nil {
			t.Skipf("workspace symlink unavailable: %v", err)
		}

		_, err = applyOMPProfile(context.Background(), linkedRoot, "balanced", runner, noOpOMPProfileActivation)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workspace_root_unsafe")
		assert.Empty(t, runner.calls)
		after, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		assert.Equal(t, before, after)
	})

	t.Run("config", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "workspace")
		configPath, runner := writeOMPApplySecurityFixture(t, root)
		realConfig := filepath.Join(root, "real-autopus.yaml")
		require.NoError(t, os.Rename(configPath, realConfig))
		if err := os.Symlink(realConfig, configPath); err != nil {
			t.Skipf("config symlink unavailable: %v", err)
		}
		before, err := os.ReadFile(realConfig)
		require.NoError(t, err)

		_, err = applyOMPProfile(context.Background(), root, "balanced", runner, noOpOMPProfileActivation)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "autopus_config_unsafe")
		assert.Empty(t, runner.calls)
		after, readErr := os.ReadFile(realConfig)
		require.NoError(t, readErr)
		assert.Equal(t, before, after)
	})
}

func TestApplyOMPProfileFailsClosedWhenWorkspaceOrConfigSwapsDuringActivation(t *testing.T) {
	t.Run("workspace", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "workspace")
		_, runner := writeOMPApplySecurityFixture(t, root)
		movedRoot := filepath.Join(parent, "moved")
		attacker := []byte("platforms: [omp]\nattacker: true\n")

		_, err := applyOMPProfile(
			context.Background(), root, "balanced", runner,
			func(context.Context, string, *config.HarnessConfig) error {
				require.NoError(t, os.Rename(root, movedRoot))
				require.NoError(t, os.Mkdir(root, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(root, "autopus.yaml"), attacker, 0o640))
				return nil
			},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "autopus_config_changed_during_activation")
		after, readErr := os.ReadFile(filepath.Join(root, "autopus.yaml"))
		require.NoError(t, readErr)
		assert.Equal(t, attacker, after)
	})

	t.Run("config", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "workspace")
		configPath, runner := writeOMPApplySecurityFixture(t, root)
		movedConfig := filepath.Join(root, "applied-autopus.yaml")
		attacker := []byte("platforms: [omp]\nattacker: true\n")

		_, err := applyOMPProfile(
			context.Background(), root, "balanced", runner,
			func(context.Context, string, *config.HarnessConfig) error {
				require.NoError(t, os.Rename(configPath, movedConfig))
				require.NoError(t, os.WriteFile(configPath, attacker, 0o640))
				return nil
			},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "autopus_config_changed_during_activation")
		after, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		assert.Equal(t, attacker, after)
	})
}

func writeOMPApplySecurityFixture(t *testing.T, root string) (string, *ompCLIFakeRunner) {
	t.Helper()
	require.NoError(t, os.Mkdir(root, 0o700))
	cfg := config.DefaultFullConfig("omp-security")
	cfg.Platforms = []string{"omp"}
	require.NoError(t, config.Save(root, cfg))
	configPath := filepath.Join(root, "autopus.yaml")
	require.NoError(t, os.Chmod(configPath, 0o640))
	return configPath, &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
}

func noOpOMPProfileActivation(context.Context, string, *config.HarnessConfig) error {
	return nil
}

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestPlatformCatalog_ContainsEverySupportedLifecycle(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		[]string{"claude-code", "codex", "antigravity-cli", "opencode", "omp"},
		platformCatalogNames(),
	)
	for _, name := range platformCatalogNames() {
		descriptor, ok := lookupPlatformDescriptor(name)
		require.True(t, ok, "catalog entry %q", name)
		require.NotNil(t, descriptor.newAdapter, "adapter factory %q", name)
	}
	_, ok := lookupPlatformDescriptor("unknown")
	assert.False(t, ok)
}

func TestGeneratePlatformThenSave_GenerationFailureLeavesConfigUncommitted(t *testing.T) {
	t.Parallel()

	generationErr := errors.New("generation failed")
	descriptor := platformDescriptor{
		name: "failing",
		newAdapter: func(string, platformAdapterOptions) adapter.PlatformAdapter {
			return &lifecycleTestAdapter{name: "failing", generateErr: generationErr}
		},
	}
	cfg := config.DefaultFullConfig("commit-order")
	cfg.Platforms = []string{"claude-code", "failing"}
	saveCalls := 0

	err := generatePlatformThenSave(context.Background(), t.TempDir(), descriptor, cfg, cfg,
		func(string, *config.HarnessConfig) error {
			saveCalls++
			return nil
		})

	require.ErrorIs(t, err, generationErr)
	assert.Zero(t, saveCalls, "config must only be saved after generation succeeds")
}

func TestCleanPlatformThenSave_CleanupFailureLeavesConfigUncommitted(t *testing.T) {
	t.Parallel()

	cleanupErr := errors.New("cleanup failed")
	descriptor := platformDescriptor{
		name: "failing",
		newAdapter: func(string, platformAdapterOptions) adapter.PlatformAdapter {
			return &lifecycleTestAdapter{name: "failing", cleanErr: cleanupErr}
		},
	}
	cfg := config.DefaultFullConfig("commit-order")
	cfg.Platforms = []string{"claude-code"}
	saveCalls := 0

	_, err := cleanPlatformThenSave(context.Background(), t.TempDir(), descriptor, cfg,
		func(string, *config.HarnessConfig) error {
			saveCalls++
			return nil
		})

	require.ErrorIs(t, err, cleanupErr)
	assert.Zero(t, saveCalls, "config must only be saved after cleanup succeeds")
}

func TestUpdatePlatformThenSave_UpdateFailureLeavesConfigUncommitted(t *testing.T) {
	t.Parallel()

	updateErr := errors.New("update failed")
	descriptor := platformDescriptor{
		name: "failing",
		newAdapter: func(string, platformAdapterOptions) adapter.PlatformAdapter {
			return &lifecycleTestAdapter{name: "failing", updateErr: updateErr}
		},
	}
	cfg := config.DefaultFullConfig("commit-order")
	cfg.Platforms = []string{"claude-code", "failing"}
	saveCalls := 0

	err := updatePlatformThenSave(context.Background(), t.TempDir(), descriptor, cfg, cfg,
		func(string, *config.HarnessConfig) error {
			saveCalls++
			return nil
		})

	require.ErrorIs(t, err, updateErr)
	assert.Zero(t, saveCalls, "detected platform config must only be saved after update succeeds")
}

func TestGeneratePlatformThenSave_SuccessOrdersGenerationBeforeCommit(t *testing.T) {
	t.Parallel()

	events := make([]string, 0, 2)
	descriptor := platformDescriptor{
		name: "ordered",
		newAdapter: func(string, platformAdapterOptions) adapter.PlatformAdapter {
			return &lifecycleTestAdapter{name: "ordered", onGenerate: func() { events = append(events, "generate") }}
		},
	}
	cfg := config.DefaultFullConfig("commit-order")

	err := generatePlatformThenSave(context.Background(), t.TempDir(), descriptor, cfg, cfg,
		func(string, *config.HarnessConfig) error {
			events = append(events, "save")
			return nil
		})

	require.NoError(t, err)
	assert.Equal(t, []string{"generate", "save"}, events)
}

func TestSaveConfigWithGenerationRollback_FailureRestoresSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	original := config.DefaultFullConfig("existing")
	original.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(root, original))
	configPath := filepath.Join(root, "autopus.yaml")
	before, err := os.ReadFile(configPath)
	require.NoError(t, err)

	candidate := config.DefaultFullConfig("candidate")
	candidate.Platforms = []string{"claude-code", "codex"}
	generationErr := errors.New("generation failed")
	err = saveConfigWithGenerationRollback(root, candidate, func() error {
		loaded, loadErr := config.LoadPreview(root)
		require.NoError(t, loadErr)
		assert.Equal(t, candidate.Platforms, loaded.Platforms)
		return generationErr
	})

	require.ErrorIs(t, err, generationErr)
	after, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, before, after, "failed init generation must restore the exact prior config")
}

func TestSaveConfigWithGenerationRollback_FailureRemovesNewConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.DefaultFullConfig("new")
	generationErr := errors.New("generation failed")

	err := saveConfigWithGenerationRollback(root, cfg, func() error { return generationErr })

	require.ErrorIs(t, err, generationErr)
	_, statErr := os.Stat(filepath.Join(root, "autopus.yaml"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestPlatformAdd_GenerationFailureDoesNotCommitCandidateConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.DefaultFullConfig("platform-add")
	cfg.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(root, cfg))
	configPath := filepath.Join(root, "autopus.yaml")
	before, err := os.ReadFile(configPath)
	require.NoError(t, err)

	generationErr := errors.New("generation failed")
	failing := platformDescriptor{
		name: "codex",
		newAdapter: func(string, platformAdapterOptions) adapter.PlatformAdapter {
			return &lifecycleTestAdapter{name: "codex", generateErr: generationErr}
		},
	}
	lookup := func(name string) (platformDescriptor, bool) {
		if name == "codex" {
			return failing, true
		}
		return lookupPlatformDescriptor(name)
	}
	dirFlag := root
	cmd := newPlatformAddCmdWithLookup(&dirFlag, lookup)
	cmd.SetArgs([]string{"codex"})

	err = cmd.Execute()

	require.ErrorIs(t, err, generationErr)
	after, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, before, after)
	reloaded, loadErr := config.LoadPreview(root)
	require.NoError(t, loadErr)
	assert.NotContains(t, reloaded.Platforms, "codex")
}

func TestPlatformRemove_CleanupFailureDoesNotCommitCandidateConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.DefaultFullConfig("platform-remove")
	cfg.Platforms = []string{"claude-code", "codex"}
	require.NoError(t, config.Save(root, cfg))
	configPath := filepath.Join(root, "autopus.yaml")
	before, err := os.ReadFile(configPath)
	require.NoError(t, err)

	cleanupErr := errors.New("cleanup failed")
	failing := platformDescriptor{
		name: "codex",
		newAdapter: func(string, platformAdapterOptions) adapter.PlatformAdapter {
			return &lifecycleTestAdapter{name: "codex", cleanErr: cleanupErr}
		},
	}
	lookup := func(name string) (platformDescriptor, bool) {
		if name == "codex" {
			return failing, true
		}
		return lookupPlatformDescriptor(name)
	}
	dirFlag := root
	cmd := newPlatformRemoveCmdWithLookup(&dirFlag, lookup)
	cmd.SetArgs([]string{"codex"})

	err = cmd.Execute()

	require.ErrorIs(t, err, cleanupErr)
	after, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, before, after)
	reloaded, loadErr := config.LoadPreview(root)
	require.NoError(t, loadErr)
	assert.Contains(t, reloaded.Platforms, "codex")
}

type lifecycleTestAdapter struct {
	name        string
	generateErr error
	updateErr   error
	cleanErr    error
	onGenerate  func()
}

func (a *lifecycleTestAdapter) Name() string                         { return a.name }
func (a *lifecycleTestAdapter) Version() string                      { return "test" }
func (a *lifecycleTestAdapter) CLIBinary() string                    { return a.name }
func (a *lifecycleTestAdapter) Detect(context.Context) (bool, error) { return true, nil }
func (a *lifecycleTestAdapter) Generate(context.Context, *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	if a.onGenerate != nil {
		a.onGenerate()
	}
	return nil, a.generateErr
}
func (a *lifecycleTestAdapter) Update(context.Context, *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	return nil, a.updateErr
}
func (a *lifecycleTestAdapter) Validate(context.Context) ([]adapter.ValidationError, error) {
	return nil, nil
}
func (a *lifecycleTestAdapter) Clean(context.Context) error { return a.cleanErr }
func (a *lifecycleTestAdapter) SupportsHooks() bool         { return false }
func (a *lifecycleTestAdapter) InstallHooks(context.Context, []adapter.HookConfig, *adapter.PermissionSet) error {
	return nil
}

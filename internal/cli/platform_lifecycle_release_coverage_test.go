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

const (
	releaseCompanionPlatform = "release-companion"
	releaseCompanionManaged  = ".agents/release-companion/managed.txt"
	releaseCompanionNew      = ".agents/release-companion/new.txt"
)

func TestReleaseLifecycle_SaveFailureRollsBackCompanionTransaction(t *testing.T) {
	t.Parallel()
	fixture := newLifecycleRollbackFixture(t)
	fixture.descriptor.rollbackPlatforms = []string{releaseCompanionPlatform, "", releaseCompanionPlatform}

	beforeFiles := releaseLifecycleFiles(releaseCompanionManaged, "before companion\n")
	baseline, err := applyReleaseLifecycleTransaction(fixture.root, releaseCompanionPlatform, beforeFiles)
	require.NoError(t, err)
	companionPath := filepath.Join(fixture.root, filepath.FromSlash(releaseCompanionManaged))
	companionNewPath := filepath.Join(fixture.root, filepath.FromSlash(releaseCompanionNew))
	companionManifestPath := filepath.Join(fixture.root, filepath.FromSlash(platformManifestPath(releaseCompanionPlatform)))
	companionBefore := readLifecycleBytes(t, companionPath)
	manifestBefore := readLifecycleBytes(t, companionManifestPath)

	saveErr := errors.New("release config save failed")
	var generated *adapter.TransactionJournal
	err = generatePlatformThenSave(
		context.Background(), fixture.root, fixture.descriptor, fixture.candidate, fixture.candidate,
		func(root string, cfg *config.HarnessConfig) error {
			changed := &adapter.PlatformFiles{Files: append(
				releaseLifecycleFiles(releaseCompanionManaged, "changed companion\n").Files,
				releaseLifecycleFile(releaseCompanionNew, "new companion\n"),
			)}
			generated, err = applyReleaseLifecycleTransaction(root, releaseCompanionPlatform, changed)
			require.NoError(t, err)
			require.NoError(t, config.Save(root, cfg))
			return saveErr
		},
	)

	assertLifecycleSaveStage(t, err, saveErr)
	var lifecycleErr *platformLifecycleError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.True(t, lifecycleErr.rolledBack)
	fixture.assertRestored(t)
	assert.Equal(t, companionBefore, readLifecycleBytes(t, companionPath))
	assert.Equal(t, manifestBefore, readLifecycleBytes(t, companionManifestPath))
	assert.NoFileExists(t, companionNewPath)
	committed, listErr := adapter.ListCommittedTransactions(fixture.root)
	require.NoError(t, listErr)
	require.Len(t, committed, 1)
	assert.Equal(t, baseline.ID, committed[0].ID)
	assert.NotEqual(t, baseline.ID, generated.ID)
}

func TestReleaseLifecycle_PartialGenerationFailureRestoresSurface(t *testing.T) {
	t.Parallel()
	fixture := newLifecycleRollbackFixture(t)
	generationErr := errors.New("generation stopped after writes")
	fixture.descriptor.newAdapter = func(root string, _ platformAdapterOptions) adapter.PlatformAdapter {
		return &releasePartialGenerateAdapter{
			lifecycleRollbackAdapter: &lifecycleRollbackAdapter{root: root},
			err:                      generationErr,
		}
	}
	saveCalls := 0

	err := generatePlatformThenSave(
		context.Background(), fixture.root, fixture.descriptor, fixture.candidate, fixture.candidate,
		func(string, *config.HarnessConfig) error {
			saveCalls++
			return nil
		},
	)

	require.ErrorIs(t, err, generationErr)
	var lifecycleErr *platformLifecycleError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.Equal(t, platformStageGenerate, lifecycleErr.stage)
	assert.True(t, lifecycleErr.rolledBack)
	assert.Zero(t, saveCalls)
	fixture.assertRestored(t)
}

func TestReleaseLifecycle_CleanFailureClearsReceiptAfterRollback(t *testing.T) {
	t.Parallel()
	fixture := newLifecycleRollbackFixture(t)
	cleanupErr := errors.New("cleanup stopped after deletes")
	fixture.descriptor.clean = func(context.Context, string) (platformCleanReceipt, error) {
		receipt := platformCleanReceipt{changedPaths: []string{lifecycleManagedPath}}
		require.NoError(t, os.Remove(fixture.managedPath))
		require.NoError(t, os.Remove(fixture.manifestPath))
		return receipt, cleanupErr
	}
	saveCalls := 0

	receipt, err := cleanPlatformThenSave(
		context.Background(), fixture.root, fixture.descriptor, fixture.candidate,
		func(string, *config.HarnessConfig) error {
			saveCalls++
			return nil
		},
	)

	require.ErrorIs(t, err, cleanupErr)
	var lifecycleErr *platformLifecycleError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.Equal(t, platformStageClean, lifecycleErr.stage)
	assert.True(t, lifecycleErr.rolledBack)
	assert.Empty(t, receipt.changedPaths)
	assert.Zero(t, saveCalls)
	fixture.assertRestored(t)
}

func TestReleaseLifecycle_UnsafeGeneratedPathReportsIncompleteRollback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	original := config.DefaultFullConfig("before")
	require.NoError(t, config.Save(root, original))
	configBefore := readLifecycleBytes(t, filepath.Join(root, "autopus.yaml"))
	outsidePath := filepath.Join(filepath.Dir(root), "outside.txt")
	require.NoError(t, os.WriteFile(outsidePath, []byte("outside sentinel\n"), 0o600))

	unsafeTarget := filepath.ToSlash(filepath.Join("..", filepath.Base(outsidePath)))
	descriptor := platformDescriptor{
		name: "release-unsafe",
		newAdapter: func(string, platformAdapterOptions) adapter.PlatformAdapter {
			return &releaseUnsafePathAdapter{
				lifecycleTestAdapter: &lifecycleTestAdapter{name: "release-unsafe"},
				target:               unsafeTarget,
			}
		},
	}
	candidate := config.DefaultFullConfig("after")
	saveErr := errors.New("save failed after unsafe result")

	err := generatePlatformThenSave(
		context.Background(), root, descriptor, candidate, candidate,
		func(root string, cfg *config.HarnessConfig) error {
			require.NoError(t, config.Save(root, cfg))
			return saveErr
		},
	)

	require.ErrorIs(t, err, saveErr)
	assert.ErrorContains(t, err, "platform lifecycle rollback failed")
	var lifecycleErr *platformLifecycleError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.Equal(t, platformStageSave, lifecycleErr.stage)
	assert.False(t, lifecycleErr.rolledBack)
	assert.Equal(t, configBefore, readLifecycleBytes(t, filepath.Join(root, "autopus.yaml")))
	assert.Equal(t, []byte("outside sentinel\n"), readLifecycleBytes(t, outsidePath))
}

func TestReleaseLifecycle_SuccessCommitsConfigAndGeneratedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	candidate := config.DefaultFullConfig("release-success")
	generatedPath := filepath.Join(root, "CLAUDE.md")

	err := saveConfigWithGenerationRollback(root, candidate, func() error {
		return os.WriteFile(generatedPath, []byte("committed release surface\n"), 0o640)
	})

	require.NoError(t, err)
	loaded, loadErr := config.LoadPreview(root)
	require.NoError(t, loadErr)
	assert.Equal(t, candidate.ProjectName, loaded.ProjectName)
	assert.Equal(t, candidate.Platforms, loaded.Platforms)
	assert.Equal(t, []byte("committed release surface\n"), readLifecycleBytes(t, generatedPath))
}

type releasePartialGenerateAdapter struct {
	*lifecycleRollbackAdapter
	err error
}

func (a *releasePartialGenerateAdapter) Generate(
	ctx context.Context,
	cfg *config.HarnessConfig,
) (*adapter.PlatformFiles, error) {
	files, err := a.lifecycleRollbackAdapter.Generate(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return files, a.err
}

type releaseUnsafePathAdapter struct {
	*lifecycleTestAdapter
	target string
}

func (a *releaseUnsafePathAdapter) Generate(
	context.Context,
	*config.HarnessConfig,
) (*adapter.PlatformFiles, error) {
	return releaseLifecycleFiles(a.target, "unsafe\n"), nil
}

func releaseLifecycleFile(path, content string) adapter.FileMapping {
	return adapter.FileMapping{
		TargetPath:      path,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        adapter.Checksum(content),
		Content:         []byte(content),
	}
}

func releaseLifecycleFiles(path, content string) *adapter.PlatformFiles {
	return &adapter.PlatformFiles{Files: []adapter.FileMapping{releaseLifecycleFile(path, content)}}
}

func applyReleaseLifecycleTransaction(
	root string,
	platform string,
	files *adapter.PlatformFiles,
) (*adapter.TransactionJournal, error) {
	writes := adapter.TransactionWritesFromFiles(files.Files, func(string) os.FileMode { return 0o640 })
	return adapter.ApplyTransaction(root, platform, adapter.TransactionPlan{
		Writes:   writes,
		Manifest: adapter.ManifestFromFiles(platform, files),
	})
}

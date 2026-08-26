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
	lifecycleRollbackPlatform = "rollback-test"
	lifecycleManagedPath      = ".agents/rollback-test/managed.txt"
	lifecycleNewManagedPath   = ".agents/rollback-test/new.txt"
	lifecycleUserPath         = ".agents/rollback-test/user.txt"
)

func TestGeneratePlatformThenSave_SaveFailureRestoresLifecycleSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newLifecycleRollbackFixture(t)
	saveErr := errors.New("save failed")

	err := generatePlatformThenSave(
		context.Background(), fixture.root, fixture.descriptor, fixture.candidate, fixture.candidate,
		failingLifecycleSave(t, fixture, false, saveErr),
	)

	assertLifecycleSaveStage(t, err, saveErr)
	fixture.assertRestored(t)
}

func TestUpdatePlatformThenSave_SaveFailureRestoresLifecycleSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newLifecycleRollbackFixture(t)
	saveErr := errors.New("save failed")

	err := updatePlatformThenSave(
		context.Background(), fixture.root, fixture.descriptor, fixture.candidate, fixture.candidate,
		failingLifecycleSave(t, fixture, false, saveErr),
	)

	assertLifecycleSaveStage(t, err, saveErr)
	fixture.assertRestored(t)
}

func TestCleanPlatformThenSave_SaveFailureRestoresLifecycleSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newLifecycleRollbackFixture(t)
	saveErr := errors.New("save failed")

	_, err := cleanPlatformThenSave(
		context.Background(), fixture.root, fixture.descriptor, fixture.candidate,
		failingLifecycleSave(t, fixture, true, saveErr),
	)

	assertLifecycleSaveStage(t, err, saveErr)
	fixture.assertRestored(t)
}

func TestPlatformLifecycleThenSave_SuccessKeepsCommittedSurface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(context.Context, *lifecycleRollbackFixture) error
		clean bool
	}{
		{
			name: "generate",
			apply: func(ctx context.Context, fixture *lifecycleRollbackFixture) error {
				return generatePlatformThenSave(
					ctx, fixture.root, fixture.descriptor, fixture.candidate, fixture.candidate, config.Save,
				)
			},
		},
		{
			name: "update",
			apply: func(ctx context.Context, fixture *lifecycleRollbackFixture) error {
				return updatePlatformThenSave(
					ctx, fixture.root, fixture.descriptor, fixture.candidate, fixture.candidate, config.Save,
				)
			},
		},
		{
			name: "clean",
			apply: func(ctx context.Context, fixture *lifecycleRollbackFixture) error {
				_, err := cleanPlatformThenSave(
					ctx, fixture.root, fixture.descriptor, fixture.candidate, config.Save,
				)
				return err
			},
			clean: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fixture := newLifecycleRollbackFixture(t)

			require.NoError(t, tt.apply(context.Background(), fixture))

			configAfter := readLifecycleBytes(t, fixture.configPath)
			assert.NotEqual(t, fixture.configBefore, configAfter)
			assert.Equal(t, fixture.userBefore, readLifecycleBytes(t, fixture.userPath))
			if tt.clean {
				assert.NoFileExists(t, fixture.managedPath)
				assert.NoFileExists(t, fixture.manifestPath)
				return
			}
			assert.Equal(t, []byte("changed\n"), readLifecycleBytes(t, fixture.managedPath))
			assert.FileExists(t, fixture.newManagedPath)
			assert.NotEqual(t, fixture.manifestBefore, readLifecycleBytes(t, fixture.manifestPath))
		})
	}
}

type lifecycleRollbackFixture struct {
	root           string
	descriptor     platformDescriptor
	candidate      *config.HarnessConfig
	configPath     string
	manifestPath   string
	managedPath    string
	newManagedPath string
	userPath       string
	configBefore   []byte
	manifestBefore []byte
	managedBefore  []byte
	userBefore     []byte
}

func newLifecycleRollbackFixture(t *testing.T) *lifecycleRollbackFixture {
	t.Helper()

	root := t.TempDir()
	original := config.DefaultFullConfig("before")
	original.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(root, original))

	managedPath := filepath.Join(root, filepath.FromSlash(lifecycleManagedPath))
	userPath := filepath.Join(root, filepath.FromSlash(lifecycleUserPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(managedPath), 0o755))
	require.NoError(t, os.WriteFile(managedPath, []byte("before\n"), 0o640))
	require.NoError(t, os.WriteFile(userPath, []byte("user\n"), 0o600))

	files := &adapter.PlatformFiles{Files: []adapter.FileMapping{{
		TargetPath:      lifecycleManagedPath,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        adapter.Checksum("before\n"),
	}}}
	require.NoError(t, adapter.ManifestFromFiles(lifecycleRollbackPlatform, files).Save(root))

	candidate := config.DefaultFullConfig("after")
	candidate.Platforms = []string{"claude-code", "codex"}
	fixture := &lifecycleRollbackFixture{
		root:           root,
		candidate:      candidate,
		configPath:     filepath.Join(root, "autopus.yaml"),
		manifestPath:   filepath.Join(root, ".autopus", lifecycleRollbackPlatform+"-manifest.json"),
		managedPath:    managedPath,
		newManagedPath: filepath.Join(root, filepath.FromSlash(lifecycleNewManagedPath)),
		userPath:       userPath,
	}
	fixture.descriptor = platformDescriptor{
		name: lifecycleRollbackPlatform,
		newAdapter: func(string, platformAdapterOptions) adapter.PlatformAdapter {
			return &lifecycleRollbackAdapter{root: root}
		},
	}
	fixture.configBefore = readLifecycleBytes(t, fixture.configPath)
	fixture.manifestBefore = readLifecycleBytes(t, fixture.manifestPath)
	fixture.managedBefore = readLifecycleBytes(t, fixture.managedPath)
	fixture.userBefore = readLifecycleBytes(t, fixture.userPath)
	return fixture
}

func (f *lifecycleRollbackFixture) assertRestored(t *testing.T) {
	t.Helper()
	assert.Equal(t, f.configBefore, readLifecycleBytes(t, f.configPath))
	assert.Equal(t, f.manifestBefore, readLifecycleBytes(t, f.manifestPath))
	assert.Equal(t, f.managedBefore, readLifecycleBytes(t, f.managedPath))
	assert.Equal(t, f.userBefore, readLifecycleBytes(t, f.userPath))
	assert.NoFileExists(t, f.newManagedPath)
}

func failingLifecycleSave(
	t *testing.T,
	fixture *lifecycleRollbackFixture,
	clean bool,
	saveErr error,
) platformConfigSaver {
	t.Helper()
	return func(root string, cfg *config.HarnessConfig) error {
		if clean {
			assert.NoFileExists(t, fixture.managedPath)
			assert.NoFileExists(t, fixture.manifestPath)
		} else {
			assert.Equal(t, []byte("changed\n"), readLifecycleBytes(t, fixture.managedPath))
			assert.FileExists(t, fixture.newManagedPath)
			assert.NotEqual(t, fixture.manifestBefore, readLifecycleBytes(t, fixture.manifestPath))
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		return saveErr
	}
}

func assertLifecycleSaveStage(t *testing.T, err, saveErr error) {
	t.Helper()
	require.ErrorIs(t, err, saveErr)
	var lifecycleErr *platformLifecycleError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.Equal(t, platformStageSave, lifecycleErr.stage)
}

func readLifecycleBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

type lifecycleRollbackAdapter struct {
	root string
}

func (a *lifecycleRollbackAdapter) Name() string    { return lifecycleRollbackPlatform }
func (a *lifecycleRollbackAdapter) Version() string { return "test" }
func (a *lifecycleRollbackAdapter) CLIBinary() string {
	return lifecycleRollbackPlatform
}
func (a *lifecycleRollbackAdapter) Detect(context.Context) (bool, error) { return true, nil }
func (a *lifecycleRollbackAdapter) Generate(context.Context, *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	return a.writeSurface()
}
func (a *lifecycleRollbackAdapter) Update(context.Context, *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	return a.writeSurface()
}
func (a *lifecycleRollbackAdapter) Validate(context.Context) ([]adapter.ValidationError, error) {
	return nil, nil
}
func (a *lifecycleRollbackAdapter) Clean(context.Context) error {
	if err := os.Remove(filepath.Join(a.root, filepath.FromSlash(lifecycleManagedPath))); err != nil {
		return err
	}
	return os.Remove(filepath.Join(a.root, ".autopus", lifecycleRollbackPlatform+"-manifest.json"))
}
func (a *lifecycleRollbackAdapter) SupportsHooks() bool { return false }
func (a *lifecycleRollbackAdapter) InstallHooks(context.Context, []adapter.HookConfig, *adapter.PermissionSet) error {
	return nil
}

func (a *lifecycleRollbackAdapter) writeSurface() (*adapter.PlatformFiles, error) {
	files := []adapter.FileMapping{
		{
			TargetPath:      lifecycleManagedPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum("changed\n"),
			Content:         []byte("changed\n"),
		},
		{
			TargetPath:      lifecycleNewManagedPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum("new\n"),
			Content:         []byte("new\n"),
		},
	}
	for _, file := range files {
		path := filepath.Join(a.root, filepath.FromSlash(file.TargetPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, file.Content, 0o644); err != nil {
			return nil, err
		}
	}
	platformFiles := &adapter.PlatformFiles{Files: files}
	if err := adapter.ManifestFromFiles(lifecycleRollbackPlatform, platformFiles).Save(a.root); err != nil {
		return nil, err
	}
	return platformFiles, nil
}

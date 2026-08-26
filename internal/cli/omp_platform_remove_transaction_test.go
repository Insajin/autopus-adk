package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestOMP002_S6_PlatformRemovePropagatesCleanFailureBeforeConfigMutation(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("omp-remove-transaction")
	cfg.Platforms = []string{"claude-code", "omp"}
	require.NoError(t, config.Save(dir, cfg))
	_, err := omp.NewWithRoot(dir).Generate(context.Background(), cfg)
	require.NoError(t, err)

	configPath := filepath.Join(dir, "autopus.yaml")
	before, err := os.ReadFile(configPath)
	require.NoError(t, err)
	manifestPath := filepath.Join(dir, ".autopus", "omp-manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte("{not-json\n"), 0o600))

	dirFlag := dir
	cmd := newPlatformRemoveCmd(&dirFlag)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"omp"})
	err = cmd.Execute()
	require.Error(t, err, "Clean preflight/load failure must reach the caller")
	assert.NotContains(t, stdout.String(), "removed", "a failed transaction must not print success")

	after, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, before, after, "autopus.yaml must stay byte-identical when Clean cannot start")
	reloaded, loadErr := config.Load(dir)
	require.NoError(t, loadErr)
	assert.Contains(t, reloaded.Platforms, "omp", "OMP remains configured after a failed removal")
	assert.Empty(t, config.PlatformToProvider("omp"),
		"runtime ancestry or platform presence must never grant orchestra authority")
}

func TestOMP002_S6_PlatformRemoveLateSaveFailureRollsBackWithoutRepairReceipt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission fixture requires POSIX file modes")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the read-only config permission used by this fixture")
	}

	dir := t.TempDir()
	cfg := config.DefaultFullConfig("omp-remove-late-save")
	cfg.Platforms = []string{"claude-code", "omp"}
	require.NoError(t, config.Save(dir, cfg))
	_, err := claude.NewWithRoot(dir).Generate(context.Background(), cfg)
	require.NoError(t, err)
	_, err = omp.NewWithRoot(dir).Generate(context.Background(), cfg)
	require.NoError(t, err)

	configPath := filepath.Join(dir, "autopus.yaml")
	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(configPath, 0o444))
	t.Cleanup(func() { _ = os.Chmod(configPath, 0o644) })
	treeBefore := snapshotOMPRemoveFiles(t, dir)

	dirFlag := dir
	cmd := newPlatformRemoveCmd(&dirFlag)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"omp"})
	err = cmd.Execute()
	require.Error(t, err, "config save must fail after the OMP clean transaction")
	assert.NotContains(t, stdout.String(), "removed")
	assert.NotContains(t, err.Error(), "repair_required=true")
	assert.Contains(t, err.Error(), "rollback 완료")

	treeAfter := snapshotOMPRemoveFiles(t, dir)
	assert.Equal(t, treeBefore, treeAfter,
		"successful rollback must restore every file changed before Save failed")

	configAfter, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, configBefore, configAfter, "the failed Save must leave autopus.yaml byte-identical")
	reloaded, loadErr := config.LoadPreview(dir)
	require.NoError(t, loadErr)
	assert.Contains(t, reloaded.Platforms, "omp", "OMP remains configured so rerunning removal can repair the workspace")
	assert.FileExists(t, filepath.Join(dir, ".autopus", "claude-code-manifest.json"),
		"the unrelated Claude platform remains installed")
}

func snapshotOMPRemoveFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !entry.Type().IsRegular() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	}))
	return files
}

func changedOMPRemoveFiles(before, after map[string]string) []string {
	var changed []string
	for path, content := range before {
		if next, exists := after[path]; !exists || next != content {
			changed = append(changed, path)
		}
	}
	for path := range after {
		if _, exists := before[path]; !exists {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func parseOMPRepairPaths(t *testing.T, message string) []string {
	t.Helper()
	const marker = "changed_paths=["
	start := strings.Index(message, marker)
	require.NotEqual(t, -1, start, "repair receipt must include changed_paths")
	remainder := message[start+len(marker):]
	end := strings.IndexByte(remainder, ']')
	require.NotEqual(t, -1, end, "repair receipt must close changed_paths")
	if remainder[:end] == "" {
		return nil
	}
	return strings.Split(remainder[:end], ",")
}

func uniqueOMPRemovePaths(paths []string) map[string]bool {
	unique := make(map[string]bool, len(paths))
	for _, path := range paths {
		unique[path] = true
	}
	return unique
}

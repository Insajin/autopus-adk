package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandGoCacheIsUniqueAndCleanupPreservesModules(t *testing.T) {
	projectDir := t.TempDir()
	first, err := prepareCommandGoCache(projectDir)
	require.NoError(t, err)
	second, err := prepareCommandGoCache(projectDir)
	require.NoError(t, err)
	t.Cleanup(first.Cleanup)
	t.Cleanup(second.Cleanup)

	require.NotEqual(t, first.Paths.GoBuild, second.Paths.GoBuild)
	moduleSentinel := filepath.Join(first.Paths.GoMod, "keep")
	secondSentinel := filepath.Join(second.Paths.GoBuild, "keep")
	require.NoError(t, os.WriteFile(moduleSentinel, []byte("module"), 0o644))
	require.NoError(t, os.WriteFile(secondSentinel, []byte("active"), 0o644))

	first.Cleanup()
	assert.NoDirExists(t, first.Paths.GoBuild)
	assert.FileExists(t, moduleSentinel)
	assert.FileExists(t, secondSentinel)
}

func TestCommandGoCacheFallsBackWithoutFollowingManagedSymlink(t *testing.T) {
	projectDir := t.TempDir()
	external := t.TempDir()
	cacheParent := filepath.Join(projectDir, ".autopus", "qa")
	require.NoError(t, os.MkdirAll(cacheParent, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(external, "keep"), []byte("safe"), 0o644))
	if err := os.Symlink(external, filepath.Join(cacheParent, "cache")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	cache, err := prepareCommandGoCache(projectDir)
	require.NoError(t, err)
	t.Cleanup(cache.Cleanup)
	assert.NotEqual(t, filepath.Join(projectDir, ".autopus", "qa", "cache"), cache.Paths.Root)
	assert.False(t, strings.HasPrefix(cache.Paths.GoBuild, external+string(filepath.Separator)))
	assert.False(t, strings.HasPrefix(cache.Paths.GoMod, external+string(filepath.Separator)))
	require.NoError(t, os.WriteFile(filepath.Join(cache.Paths.GoBuild, "entry"), []byte("cache"), 0o644))
	assert.NoFileExists(t, filepath.Join(external, "entry"))
	assert.FileExists(t, filepath.Join(external, "keep"))
}

func TestCommandGoCacheCanonicalizesSymlinkedProjectRoot(t *testing.T) {
	realProjectDir := t.TempDir()
	linkParent := t.TempDir()
	linkedProjectDir := filepath.Join(linkParent, "project")
	if err := os.Symlink(realProjectDir, linkedProjectDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	cache, err := prepareCommandGoCache(linkedProjectDir)
	require.NoError(t, err)
	t.Cleanup(cache.Cleanup)
	canonicalProjectDir, err := filepath.EvalSymlinks(realProjectDir)
	require.NoError(t, err)
	assert.Equal(t, canonicalProjectDir, cache.Paths.ProjectDir)
	assert.True(t, strings.HasPrefix(cache.Paths.Root, canonicalProjectDir+string(filepath.Separator)))
	assert.True(t, strings.HasPrefix(cache.Paths.GoBuild, canonicalProjectDir+string(filepath.Separator)))
}

func TestCleanupStaleCommandGoCachesKeepsFreshAndForeignEntries(t *testing.T) {
	runsRoot := t.TempDir()
	stale := filepath.Join(runsRoot, "go-build-stale")
	fresh := filepath.Join(runsRoot, "go-build-fresh")
	foreign := filepath.Join(runsRoot, "keep-me")
	for _, path := range []string{stale, fresh, foreign} {
		require.NoError(t, os.Mkdir(path, 0o755))
	}
	now := time.Now()
	require.NoError(t, os.Chtimes(stale, now.Add(-staleCommandGoCacheAge-time.Hour), now.Add(-staleCommandGoCacheAge-time.Hour)))

	cleanupStaleCommandGoCaches(runsRoot, now)
	assert.NoDirExists(t, stale)
	assert.DirExists(t, fresh)
	assert.DirExists(t, foreign)
}

func TestManagedQAGoEnvWinsAfterOverrides(t *testing.T) {
	cache, err := prepareCommandGoCache(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(cache.Cleanup)
	env := authoritativeCommandEnv(cache.Paths, nil, []string{
		"GOCACHE=/tmp/global-go-build",
		"GOMODCACHE=/tmp/global-go-mod",
		"GOPATH=/tmp/global-go-path",
	})

	assert.Contains(t, env, "GOCACHE="+cache.Paths.GoBuild)
	assert.Contains(t, env, "GOMODCACHE="+cache.Paths.GoMod)
	assert.Contains(t, env, "GOPATH="+cache.Paths.GoPath)
	assert.NotContains(t, strings.Join(env, "\n"), "/tmp/global-go-")
}

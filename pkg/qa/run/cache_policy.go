package run

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const staleCommandGoCacheAge = 24 * time.Hour

type goCachePaths struct {
	ProjectDir string
	Root       string
	GoBuild    string
	GoPath     string
	GoMod      string
}

type commandGoCache struct {
	Paths   goCachePaths
	once    sync.Once
	cleanup func()
}

func (cache *commandGoCache) Cleanup() {
	if cache == nil || cache.cleanup == nil {
		return
	}
	cache.once.Do(cache.cleanup)
}

func managedGoCachePaths(projectDir string) goCachePaths {
	if absProjectDir, err := filepath.Abs(projectDir); err == nil {
		projectDir = absProjectDir
	} else {
		projectDir = filepath.Clean(projectDir)
	}
	if resolved, err := filepath.EvalSymlinks(projectDir); err == nil {
		projectDir = resolved
	}
	root := filepath.Join(projectDir, ".autopus", "qa", "cache")
	goPath := filepath.Join(root, "gopath")
	return goCachePaths{
		ProjectDir: projectDir,
		Root:       root,
		GoPath:     goPath,
		GoMod:      filepath.Join(goPath, "pkg", "mod"),
	}
}

func prepareCommandGoCache(projectDir string) (*commandGoCache, error) {
	paths := managedGoCachePaths(projectDir)
	if err := prepareManagedCacheRoot(paths); err != nil {
		return prepareFallbackCommandGoCache(paths.ProjectDir)
	}
	runsRoot := filepath.Join(paths.Root, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return prepareFallbackCommandGoCache(paths.ProjectDir)
	}
	if err := validateNoSymlinkComponents(paths.ProjectDir, runsRoot); err != nil {
		return prepareFallbackCommandGoCache(paths.ProjectDir)
	}
	cleanupStaleCommandGoCaches(runsRoot, time.Now())
	goBuild, err := os.MkdirTemp(runsRoot, "go-build-")
	if err != nil {
		return prepareFallbackCommandGoCache(paths.ProjectDir)
	}
	paths.GoBuild = goBuild
	return &commandGoCache{
		Paths: paths,
		cleanup: func() {
			_ = os.RemoveAll(goBuild)
			_ = os.Remove(runsRoot)
		},
	}, nil
}

func prepareManagedCacheRoot(paths goCachePaths) error {
	if err := validateNoSymlinkComponents(paths.ProjectDir, paths.Root); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.GoMod, 0o755); err != nil {
		return fmt.Errorf("create managed go module cache: %w", err)
	}
	return validateNoSymlinkComponents(paths.ProjectDir, paths.GoMod)
}

func prepareFallbackCommandGoCache(projectDir string) (*commandGoCache, error) {
	root, err := os.MkdirTemp("", "autopus-qamesh-cache-")
	if err != nil {
		return nil, fmt.Errorf("create fallback qa cache: %w", err)
	}
	goPath := filepath.Join(root, "gopath")
	paths := goCachePaths{
		ProjectDir: projectDir,
		Root:       root,
		GoBuild:    filepath.Join(root, "go-build"),
		GoPath:     goPath,
		GoMod:      filepath.Join(goPath, "pkg", "mod"),
	}
	if err := os.MkdirAll(paths.GoBuild, 0o755); err != nil {
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("create fallback go build cache: %w", err)
	}
	if err := os.MkdirAll(paths.GoMod, 0o755); err != nil {
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("create fallback go module cache: %w", err)
	}
	return &commandGoCache{
		Paths: paths,
		cleanup: func() {
			_ = os.RemoveAll(root)
		},
	}, nil
}

func validateNoSymlinkComponents(base, target string) error {
	relative, err := filepath.Rel(base, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("qa cache path is outside project")
	}
	current := base
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return fmt.Errorf("inspect qa cache path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("qa cache path contains symlink: %s", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("qa cache path component is not a directory: %s", current)
		}
	}
	return nil
}

func cleanupStaleCommandGoCaches(runsRoot string, now time.Time) {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "go-build-") {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) <= staleCommandGoCacheAge {
			continue
		}
		_ = os.RemoveAll(filepath.Join(runsRoot, entry.Name()))
	}
}

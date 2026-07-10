package design

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func collectPackRefs(root string, maxRefs int, pack *Pack) error {
	return walkDesignCandidateFiles(root, maxRefs*10, func(rel, _ string, _ os.FileInfo) error {
		switch {
		case isTokenRef(rel):
			pack.TokenRefs = appendUniqueLimited(pack.TokenRefs, SourceRef{Path: rel, Kind: "token_or_theme"}, maxRefs)
		case isComponentRef(rel):
			pack.ComponentRefs = appendUniqueLimited(pack.ComponentRefs, SourceRef{Path: rel, Kind: "component"}, maxRefs)
		case isScreenshotRef(rel):
			pack.ScreenshotRefs = appendUniqueLimited(pack.ScreenshotRefs, SourceRef{Path: rel, Kind: "screenshot_ref"}, maxRefs)
		}
		return nil
	})
}

func walkDesignCandidateFiles(root string, visitLimit int, fn func(rel, abs string, info os.FileInfo) error) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = evaluatedRoot
	}
	visited := 0
	return filepath.WalkDir(rootAbs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := relPath(rootAbs, path)
		if entry.IsDir() {
			if shouldSkipPackDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if visitLimit > 0 && visited >= visitLimit {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxLocalContextBytes {
			return nil
		}
		visited++
		return fn(rel, path, info)
	})
}

func shouldSkipPackDir(rel string) bool {
	clean := filepath.ToSlash(rel)
	if clean == "." {
		return false
	}
	base := strings.ToLower(filepath.Base(clean))
	switch base {
	case ".git", "node_modules", "vendor", "dist", "build", ".next", "coverage", "target":
		return true
	}
	return strings.HasPrefix(clean, ".autopus/runtime") ||
		strings.HasPrefix(clean, ".autopus/qa") ||
		strings.HasPrefix(clean, ".autopus/orchestra") ||
		strings.HasPrefix(clean, ".autopus/brainstorms") ||
		strings.HasPrefix(clean, ".autopus/design/imports")
}

func isTokenRef(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(lower)
	if strings.Contains(lower, "design-system") || strings.Contains(lower, "/tokens/") || strings.Contains(lower, "/theme/") {
		return true
	}
	return strings.Contains(base, "token") || strings.Contains(base, "theme") || strings.HasPrefix(base, "tailwind.config") || base == "globals.css"
}

func isComponentRef(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	ext := filepath.Ext(lower)
	if ext != ".tsx" && ext != ".jsx" {
		return false
	}
	return strings.Contains(lower, "/components/") || strings.Contains(lower, "/ui/") || strings.Contains(lower, "/primitives/")
}

func isScreenshotRef(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	ext := filepath.Ext(lower)
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return false
	}
	return strings.Contains(lower, "screenshot") || strings.Contains(lower, "snapshot") || strings.Contains(lower, "golden")
}

func appendLimited(refs []SourceRef, ref SourceRef, max int) []SourceRef {
	if max > 0 && len(refs) >= max {
		return refs
	}
	refs = append(refs, ref)
	sortRefs(refs)
	return refs
}

func appendUniqueLimited(refs []SourceRef, ref SourceRef, max int) []SourceRef {
	for _, existing := range refs {
		if existing.Path == ref.Path && existing.Kind == ref.Kind {
			return refs
		}
	}
	return appendLimited(refs, ref, max)
}

func sortRefs(refs []SourceRef) {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Path < refs[j].Path
	})
}

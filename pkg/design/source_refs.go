package design

import (
	"os"
	"path/filepath"
	"strings"
)

// @AX:NOTE [AUTO]: DESIGN.md source_of_truth refs are evidence inventory, not prompt-readable context.
func collectDeclaredSourceRefs(root string, maxRefs int, pack *Pack) error {
	designPath := filepath.Join(root, "DESIGN.md")
	info, err := os.Stat(designPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() > MaxLocalContextBytes {
		return nil
	}
	data, err := os.ReadFile(designPath)
	if err != nil {
		return err
	}
	for _, raw := range parseSourceOfTruth(string(data)) {
		if err := collectDeclaredSourceRef(root, strings.TrimSpace(raw), maxRefs, pack); err != nil {
			return err
		}
	}
	return nil
}

func collectDeclaredSourceRef(root, raw string, maxRefs int, pack *Pack) error {
	if raw == "" || hasParentTraversal(raw) {
		return nil
	}
	baseRaw := strings.TrimSuffix(filepath.ToSlash(raw), "/**")
	if strings.ContainsAny(baseRaw, "*?[") {
		return nil
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = evaluatedRoot
	}
	abs, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(baseRaw)))
	if err != nil {
		return err
	}
	if !isInsideRoot(rootAbs, abs) {
		return nil
	}
	if diag := rejectSensitive(rootAbs, abs, raw); diag != nil {
		return nil
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		addPackRef(relPath(rootAbs, abs), maxRefs, pack)
		return nil
	}
	return walkDesignCandidateFiles(abs, 0, func(_, fileAbs string, info os.FileInfo) error {
		if info.IsDir() {
			return nil
		}
		addPackRef(relPath(rootAbs, fileAbs), maxRefs, pack)
		return nil
	})
}

func addPackRef(rel string, maxRefs int, pack *Pack) {
	switch {
	case isTokenRef(rel):
		pack.TokenRefs = appendUniqueLimited(pack.TokenRefs, SourceRef{Path: rel, Kind: "token_or_theme", Reason: "source_of_truth"}, maxRefs)
	case isComponentRef(rel):
		pack.ComponentRefs = appendUniqueLimited(pack.ComponentRefs, SourceRef{Path: rel, Kind: "component", Reason: "source_of_truth"}, maxRefs)
	case isScreenshotRef(rel):
		pack.ScreenshotRefs = appendUniqueLimited(pack.ScreenshotRefs, SourceRef{Path: rel, Kind: "screenshot_ref", Reason: "source_of_truth"}, maxRefs)
	}
}

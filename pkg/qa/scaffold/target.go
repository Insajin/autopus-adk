package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type projectDirResolution struct {
	ProjectDir          string
	RequestedProjectDir string
	WorkspaceRoot       string
	TargetReason        string
	Warnings            []string
	SkipScaffold        bool
}

type qaTargetCandidate struct {
	AbsPath string
	RelPath string
	Score   int
	Reasons []string
}

type WorkspaceQATarget struct {
	ProjectDir string   `json:"project_dir"`
	RelPath    string   `json:"rel_path"`
	Score      int      `json:"score"`
	Reasons    []string `json:"reasons,omitempty"`
}

func DetectWorkspaceQATargets(root string) ([]WorkspaceQATarget, bool, error) {
	normalized, err := normalizeProjectDir(root)
	if err != nil {
		return nil, false, err
	}
	candidates, hasChildRepos := detectWorkspaceQATargets(normalized)
	targets := make([]WorkspaceQATarget, 0, len(candidates))
	for _, candidate := range candidates {
		targets = append(targets, WorkspaceQATarget{
			ProjectDir: candidate.AbsPath,
			RelPath:    candidate.RelPath,
			Score:      candidate.Score,
			Reasons:    append([]string(nil), candidate.Reasons...),
		})
	}
	return targets, hasChildRepos, nil
}

func HasQAScaffoldSignals(projectDir string) bool {
	normalized, err := normalizeProjectDir(projectDir)
	if err != nil {
		return false
	}
	return hasQAScaffoldSignals(normalized)
}

func resolveProjectDir(projectDir string, explicit bool) projectDirResolution {
	resolution := projectDirResolution{ProjectDir: projectDir}
	if explicit || hasQAScaffoldSignals(projectDir) {
		return resolution
	}

	candidates, hasChildRepos := detectWorkspaceQATargets(projectDir)
	if !hasChildRepos {
		return resolution
	}
	if len(candidates) == 0 {
		resolution.WorkspaceRoot = projectDir
		resolution.SkipScaffold = true
		resolution.Warnings = append(resolution.Warnings, "multi-repo workspace detected, but no child repository has supported QA init signals; pass --project-dir <repo> after adding project test/build signals")
		return resolution
	}
	if len(candidates) > 1 && candidates[0].Score == candidates[1].Score {
		resolution.WorkspaceRoot = projectDir
		resolution.SkipScaffold = true
		resolution.Warnings = append(resolution.Warnings, fmt.Sprintf("multiple QA target repositories match equally (%s, %s); pass --project-dir <repo> to choose the project under test", candidates[0].RelPath, candidates[1].RelPath))
		return resolution
	}

	target := candidates[0]
	resolution.ProjectDir = target.AbsPath
	resolution.RequestedProjectDir = projectDir
	resolution.WorkspaceRoot = projectDir
	resolution.TargetReason = fmt.Sprintf("resolved QA target from workspace root to %s (%s)", target.RelPath, strings.Join(target.Reasons, ", "))
	return resolution
}

func hasQAScaffoldSignals(projectDir string) bool {
	return len(detectJourneyStarters(projectDir, false)) > 0
}

func detectWorkspaceQATargets(root string) ([]qaTargetCandidate, bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, false
	}

	candidates := []qaTargetCandidate{}
	hasChildRepos := false
	for _, entry := range entries {
		if !entry.IsDir() || shouldSkipWorkspaceChild(entry.Name()) {
			continue
		}
		child := filepath.Join(root, entry.Name())
		if !isGitCheckout(child) {
			continue
		}
		hasChildRepos = true
		candidate, ok := scoreQATarget(child, entry.Name())
		if ok {
			candidates = append(candidates, candidate)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].RelPath < candidates[j].RelPath
	})
	return candidates, hasChildRepos
}

func scoreQATarget(absPath, relPath string) (qaTargetCandidate, bool) {
	signals := detectSignals(absPath)
	score := 0
	reasons := []string{}

	if signals.HasDesktopGUI {
		score += 90
		reasons = append(reasons, "desktop GUI signals")
	}
	if signals.HasPlaywright {
		score += 30
		reasons = append(reasons, "Playwright signals")
	}
	switch signals.Stack {
	case "go":
		score += 35
		reasons = append(reasons, "Go module")
	case "node":
		score += 20
		reasons = append(reasons, "package.json")
	case "python":
		score += 30
		reasons = append(reasons, "Python test signals")
	case "rust":
		score += 30
		reasons = append(reasons, "Cargo project")
	}
	if hasScript(signals.Package, "test") {
		score += 35
		reasons = append(reasons, "package test script")
	}
	for _, script := range []string{"release:dry-run", "release:qa", "test:desktop-fast", "build"} {
		if hasScript(signals.Package, script) {
			score += 15
			reasons = append(reasons, "package "+script+" script")
			break
		}
	}
	lower := strings.ToLower(relPath)
	if strings.Contains(lower, "desktop") {
		score += 20
		reasons = append(reasons, "desktop repo name")
	}
	if strings.Contains(lower, "adk") || strings.Contains(lower, "harness") {
		score -= 10
	}
	if score <= 0 {
		return qaTargetCandidate{}, false
	}
	return qaTargetCandidate{
		AbsPath: absPath,
		RelPath: filepath.ToSlash(relPath),
		Score:   score,
		Reasons: uniqueStrings(reasons),
	}, true
}

func shouldSkipWorkspaceChild(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "dist", "build", "target", "tmp", "temp":
		return true
	default:
		return false
	}
}

func isGitCheckout(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

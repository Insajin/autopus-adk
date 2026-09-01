package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	qaproject "github.com/insajin/autopus-adk/pkg/qa/project"
)

// canaryUnitMaxDepth bounds workspace discovery. Depth 2 reaches the members of
// an umbrella checkout (an <umbrella>/backend + <umbrella>/frontend pair) while
// staying shallow enough that a monorepo scan is a handful of stat calls.
// Discovery never descends into a directory that already owns a stack marker,
// so an ordinary single-stack repo is resolved without any scan at all.
const canaryUnitMaxDepth = 2

// canaryStackOrder mirrors pkg/qa/scaffold's fastLaneStackOrder. Canary stdout
// is captured verbatim as QAMESH evidence, so target order must be pinned
// rather than derived from directory reads.
var canaryStackOrder = []string{"go", "node", "python", "rust"}

// canarySkipDirs carry stack markers without ever being a buildable unit of the
// project under test. Building a vendored or generated tree would report a
// third party's failure as the project's canary verdict.
var canarySkipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"testdata":     true,
}

// canaryBuildUnit is one directory of the project that declares a buildable
// stack. Rel is slash-separated and relative to the project root ("." for the
// root itself) so target IDs stay stable across machines.
type canaryBuildUnit struct {
	Rel    string
	Dir    string
	Stacks []string
}

// canaryBuildUnits resolves what the project actually contains. The project
// root wins outright when it declares a stack: only a root with nothing
// buildable of its own is treated as an umbrella checkout worth scanning.
func canaryBuildUnits(projectDir string) []canaryBuildUnit {
	if stacks := canaryUnitStacks(projectDir); len(stacks) > 0 {
		return []canaryBuildUnit{{Rel: ".", Dir: projectDir, Stacks: stacks}}
	}
	return canaryDiscoverUnits(projectDir, projectDir, 1)
}

func canaryDiscoverUnits(root, dir string, depth int) []canaryBuildUnit {
	if depth > canaryUnitMaxDepth {
		return nil
	}
	// os.ReadDir sorts by filename, which is the ordering guarantee the evidence
	// contract needs; entry order must not vary between runs.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var units []canaryBuildUnit
	for _, entry := range entries {
		name := entry.Name()
		// Symlinked directories report IsDir() false, which also keeps discovery
		// free of cycles.
		if !entry.IsDir() || strings.HasPrefix(name, ".") || canarySkipDirs[name] {
			continue
		}
		child := filepath.Join(dir, name)
		rel, relErr := filepath.Rel(root, child)
		if relErr != nil {
			continue
		}
		if stacks := canaryUnitStacks(child); len(stacks) > 0 {
			units = append(units, canaryBuildUnit{Rel: filepath.ToSlash(rel), Dir: child, Stacks: stacks})
			// A marker-bearing directory owns its subtree; descending would
			// rediscover its own examples and sample apps as separate units.
			continue
		}
		units = append(units, canaryDiscoverUnits(root, child, depth+1)...)
	}
	return units
}

// canaryUnitStacks reports every stack whose marker file is present, using the
// same marker set pkg/qa/scaffold detects fast lanes from.
func canaryUnitStacks(dir string) []string {
	stacks := make([]string, 0, len(canaryStackOrder))
	if canaryFileExists(dir, "go.mod") {
		stacks = append(stacks, "go")
	}
	if canaryFileExists(dir, "package.json") {
		stacks = append(stacks, "node")
	}
	if canaryFileExists(dir, "pyproject.toml") || canaryFileExists(dir, "requirements.txt") || canaryFileExists(dir, "pytest.ini") {
		stacks = append(stacks, "python")
	}
	if canaryFileExists(dir, "Cargo.toml") {
		stacks = append(stacks, "rust")
	}
	return stacks
}

// canaryBrowserTarget picks the directory the remote browser probe runs from:
// the probe needs a node project whose dependency tree can resolve playwright.
// The returned reason is non-empty exactly when no such directory exists.
func canaryBrowserTarget(projectDir string, units []canaryBuildUnit) (string, string) {
	if canaryFileExists(projectDir, "package.json") && qaproject.HasBrowserSignals(projectDir) {
		return projectDir, ""
	}
	for _, unit := range units {
		if !canaryUnitHasStack(unit, "node") {
			continue
		}
		if qaproject.HasBrowserSignals(unit.Dir) {
			return unit.Dir, ""
		}
	}
	return "", "no browser project detected (no package.json declaring a browser framework)"
}

func canaryUnitHasStack(unit canaryBuildUnit, want string) bool {
	for _, stack := range unit.Stacks {
		if stack == want {
			return true
		}
	}
	return false
}

// canaryPackageManager mirrors pkg/qa/scaffold's lockfile precedence so the
// canary build invokes the same package manager the QA scaffold writes into
// Journey Packs.
func canaryPackageManager(dir string) string {
	switch {
	case canaryFileExists(dir, "pnpm-lock.yaml"):
		return "pnpm"
	case canaryFileExists(dir, "yarn.lock"):
		return "yarn"
	default:
		return "npm"
	}
}

// canaryPackageScripts returns the package.json scripts block. A malformed or
// unreadable manifest yields no scripts, which downgrades the node build to a
// stated skip rather than a failure the project cannot act on.
func canaryPackageScripts(dir string) map[string]string {
	body, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil
	}
	return manifest.Scripts
}

func canaryFileExists(dir, rel string) bool {
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil && !info.IsDir()
}

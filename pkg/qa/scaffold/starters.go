package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"

	qaproject "github.com/insajin/autopus-adk/pkg/qa/project"
)

const (
	workflowNone          = "none"
	workflowGitHubActions = "github-actions"
)

type starterFile struct {
	ID              string
	RelPath         string
	Reason          string
	Body            string
	ValidateJourney bool
}

type projectSignals struct {
	Stack          string
	Package        packageManifest
	HasPackage     bool
	HasPlaywright  bool
	HasDesktopGUI  bool
	PackageManager string
}

type packageManifest struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func detectJourneyStarters(projectDir string, release bool) []starterFile {
	signals := detectSignals(projectDir)
	starters := []starterFile{}
	if fast, ok := fastStarter(signals); ok {
		starters = append(starters, fast)
	}
	if signals.HasPlaywright {
		starters = append(starters, browserStagingStarter(signals))
	}
	if signals.HasDesktopGUI {
		if desktop, ok := desktopNativeStarter(signals); ok {
			starters = append(starters, desktop)
		}
		starters = append(starters, desktopGUIStarter())
	}
	if release {
		starters = append(starters, canaryStarter())
	}
	return starters
}

func detectSignals(projectDir string) projectSignals {
	signals := projectSignals{
		HasDesktopGUI: qaproject.HasDesktopGUISignals(projectDir),
		Package: packageManifest{
			Scripts:         map[string]string{},
			Dependencies:    map[string]string{},
			DevDependencies: map[string]string{},
		},
		PackageManager: detectPackageManager(projectDir),
	}
	if exists(projectDir, "go.mod") {
		signals.Stack = "go"
	}
	if exists(projectDir, "package.json") {
		signals.Stack = "node"
		signals.HasPackage = true
		signals.Package = readPackage(projectDir)
	}
	if signals.Stack == "" && (exists(projectDir, "pyproject.toml") || exists(projectDir, "requirements.txt") || exists(projectDir, "pytest.ini")) {
		signals.Stack = "python"
	}
	if signals.Stack == "" && exists(projectDir, "Cargo.toml") {
		signals.Stack = "rust"
	}
	signals.HasPlaywright = hasPlaywright(projectDir, signals.Package)
	return signals
}

func fastStarter(signals projectSignals) (starterFile, bool) {
	switch signals.Stack {
	case "go":
		return journeyStarter("go-fast", "Go fast test lane", "cli", []string{"fast"}, "go-test", []string{"go", "test", "./..."}, "Go module detected"), true
	case "node":
		return nodeFastStarter(signals)
	case "python":
		return journeyStarter("python-fast", "Python fast test lane", "cli", []string{"fast"}, "pytest", []string{"python", "-m", "pytest"}, "Python test signals detected"), true
	case "rust":
		return journeyStarter("rust-fast", "Rust fast test lane", "cli", []string{"fast"}, "cargo-test", []string{"cargo", "test"}, "Cargo project detected"), true
	default:
		return starterFile{}, false
	}
}

func nodeFastStarter(signals projectSignals) (starterFile, bool) {
	pm := nodeCommand(signals.PackageManager)
	if hasScript(signals.Package, "test") {
		return journeyStarter("node-fast", "Node fast test lane", "package", []string{"fast"}, "node-script", []string{pm, "test"}, "package.json test script detected"), true
	}
	if hasFileSignal("vitest", signals.Package) {
		return journeyStarter("vitest-fast", "Vitest fast test lane", "frontend", []string{"fast"}, "vitest", []string{pm, "exec", "vitest", "run"}, "Vitest signals detected"), true
	}
	if hasDependency(signals.Package, "jest") {
		return journeyStarter("jest-fast", "Jest fast test lane", "frontend", []string{"fast"}, "jest", []string{pm, "exec", "jest"}, "Jest dependency detected"), true
	}
	if hasScript(signals.Package, "build") {
		return journeyStarter("node-build-fast", "Node build fast lane", "package", []string{"fast"}, "node-script", []string{pm, "run", "build"}, "package.json build script detected"), true
	}
	return starterFile{}, false
}

func browserStagingStarter(signals projectSignals) starterFile {
	pm := nodeCommand(signals.PackageManager)
	return journeyStarter("browser-staging-playwright", "Browser staging Playwright lane", "frontend", []string{"browser-staging"}, "playwright", []string{pm, "exec", "playwright", "test"}, "Playwright signals detected")
}

func desktopNativeStarter(signals projectSignals) (starterFile, bool) {
	pm := nodeCommand(signals.PackageManager)
	for _, script := range []string{"release:dry-run", "release:qa", "test:desktop-fast", "build"} {
		if hasScript(signals.Package, script) {
			return journeyStarter("desktop-native", "Desktop native release lane", "desktop", []string{"desktop-native"}, "node-script", []string{pm, "run", script}, "desktop package script detected"), true
		}
	}
	return starterFile{}, false
}

func journeyStarter(id, title, surface string, lanes []string, adapter string, argv []string, reason string) starterFile {
	return starterFile{
		ID:              id,
		RelPath:         filepath.ToSlash(filepath.Join(journeyRootRel, id+".yaml")),
		Reason:          reason,
		ValidateJourney: true,
		Body:            renderJourney(id, title, surface, lanes, adapter, argv),
	}
}

func readPackage(projectDir string) packageManifest {
	body, err := os.ReadFile(filepath.Join(projectDir, "package.json"))
	if err != nil {
		return packageManifest{}
	}
	var manifest packageManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return packageManifest{}
	}
	if manifest.Scripts == nil {
		manifest.Scripts = map[string]string{}
	}
	if manifest.Dependencies == nil {
		manifest.Dependencies = map[string]string{}
	}
	if manifest.DevDependencies == nil {
		manifest.DevDependencies = map[string]string{}
	}
	return manifest
}

func detectPackageManager(projectDir string) string {
	switch {
	case exists(projectDir, "pnpm-lock.yaml"):
		return "pnpm"
	case exists(projectDir, "yarn.lock"):
		return "yarn"
	default:
		return "npm"
	}
}

func nodeCommand(pm string) string {
	switch pm {
	case "pnpm", "yarn":
		return pm
	default:
		return "npm"
	}
}

func hasPlaywright(projectDir string, manifest packageManifest) bool {
	if hasDependency(manifest, "@playwright/test") || hasDependency(manifest, "playwright") {
		return true
	}
	for _, name := range []string{"playwright.config.ts", "playwright.config.js", "playwright.config.mjs", "playwright.config.cjs"} {
		if exists(projectDir, name) {
			return true
		}
	}
	return false
}

func hasFileSignal(name string, manifest packageManifest) bool {
	return hasDependency(manifest, name)
}

func hasScript(manifest packageManifest, name string) bool {
	_, ok := manifest.Scripts[name]
	return ok
}

func hasDependency(manifest packageManifest, name string) bool {
	if _, ok := manifest.Dependencies[name]; ok {
		return true
	}
	_, ok := manifest.DevDependencies[name]
	return ok
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

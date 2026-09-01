package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/qa/domainreadiness"
	qaproject "github.com/insajin/autopus-adk/pkg/qa/project"
)

const (
	workflowNone          = "none"
	workflowGitHubActions = "github-actions"
)

type starterFile struct {
	ID                    string
	RelPath               string
	Reason                string
	Body                  string
	Lanes                 []string
	ValidateJourney       bool
	ValidateDomainCatalog bool
}

type projectSignals struct {
	// Stack is the dominant stack, for consumers that can only act on one answer.
	// Stacks carries every stack detected, because a polyglot repo needs a fast
	// lane per stack rather than a single arbitrary winner.
	Stack             string
	Stacks            []string
	Package           packageManifest
	HasPackage        bool
	HasBrowser        bool
	HasPlaywright     bool
	HasDesktopGUI     bool
	HasTauriRust      bool
	HasAndroidSignals bool
	HasIOSSignals     bool
	PackageManager    string
	// BaseOrigin is the Playwright baseURL origin when the project states one, so
	// the gui-explore example in the capture README targets the real dev server
	// instead of a guess.
	BaseOrigin string
}

type packageManifest struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func detectJourneyStarters(projectDir string, release bool) []starterFile {
	signals := detectSignals(projectDir)
	starters := []starterFile{}
	starters = append(starters, fastStarters(signals)...)
	browserLane := signals.HasPlaywright || (release && signals.HasBrowser)
	if browserLane {
		starters = append(starters, browserStagingStarter(signals))
	}
	if signals.HasDesktopGUI {
		if desktop, ok := desktopNativeStarter(signals); ok {
			starters = append(starters, desktop)
		}
	}
	// No gui-explore Journey Pack is generated on purpose. Such a pack cannot pass
	// until the project supplies a read-only exploration suite: the whole test suite
	// trips the forbidden `mutation` action, and an empty selection trips the capture
	// contract's minimum-one-step rule. Since gui-explore is a `must` lane in the
	// default prelaunch profile, shipping an unrunnable pack turns the release gate
	// red, while a missing pack is a setup gap that reads as "configure this".
	// The capture README carries the pack to copy when the project opts in.
	if browserLane || signals.HasDesktopGUI {
		starters = append(starters, captureProducerStarters(signals)...)
	}
	if signals.HasAndroidSignals || signals.HasIOSSignals {
		starters = append(starters, mobileScriptedStarter(signals))
	}
	if len(starters) > 0 {
		starters = append(starters, domainReadinessCatalogStarter(projectDir))
	}
	if release {
		starters = append(starters, canaryStarter())
	}
	return starters
}

func domainReadinessCatalogStarter(projectDir string) starterFile {
	body, err := json.MarshalIndent(domainreadiness.StarterCatalogForProject(projectDir), "", "  ")
	if err != nil {
		body = []byte("{}")
	}
	body = append(body, '\n')
	return starterFile{
		ID:                    "domain-readiness-catalog",
		RelPath:               domainreadiness.DefaultCatalogPath,
		Reason:                "project QA signal detected",
		ValidateDomainCatalog: true,
		Body:                  string(body),
	}
}

func detectSignals(projectDir string) projectSignals {
	signals := projectSignals{
		HasDesktopGUI: qaproject.HasDesktopGUISignals(projectDir),
		Package: packageManifest{
			Scripts:         map[string]string{},
			Dependencies:    map[string]string{},
			DevDependencies: map[string]string{},
		},
		HasTauriRust:   exists(projectDir, "src-tauri/Cargo.toml"),
		PackageManager: detectPackageManager(projectDir),
	}
	signals.Stacks = detectStacks(projectDir)
	signals.Stack = dominantStack(signals.Stacks)
	if containsStack(signals.Stacks, "node") {
		signals.HasPackage = true
		signals.Package = readPackage(projectDir)
	}
	signals.HasBrowser = qaproject.HasBrowserSignals(projectDir)
	signals.HasPlaywright = hasPlaywright(projectDir, signals.Package)
	if signals.HasPlaywright || signals.HasBrowser {
		signals.BaseOrigin = detectBaseOrigin(projectDir)
	}
	signals.HasAndroidSignals = qaproject.HasAndroidSignals(projectDir)
	signals.HasIOSSignals = qaproject.HasIOSSignals(projectDir)
	return signals
}

func browserStagingStarter(signals projectSignals) starterFile {
	pm := nodeCommand(signals.PackageManager)
	reason := "browser app signals detected"
	if signals.HasPlaywright {
		reason = "Playwright signals detected"
	}
	return journeyStarter("browser-staging-playwright", "Browser staging Playwright lane", "frontend", []string{"browser-staging"}, "playwright", jsRunnerArgv(pm, "playwright", "test"), reason)
}

func desktopNativeStarter(signals projectSignals) (starterFile, bool) {
	pm := nodeCommand(signals.PackageManager)
	for _, script := range []string{"release:dry-run", "release:qa", "test:desktop-fast", "build"} {
		if hasScript(signals.Package, script) {
			return journeyStarter("desktop-native", "Desktop native release lane", "desktop", []string{"desktop-native"}, "node-script", []string{pm, "run", script}, "desktop package script detected"), true
		}
	}
	if signals.HasTauriRust {
		return journeyStarterWithCWD("desktop-native", "Desktop native Rust test lane", "desktop", []string{"desktop-native"}, "cargo-test", []string{"cargo", "test"}, "src-tauri", "Tauri Rust project detected"), true
	}
	return starterFile{}, false
}

func journeyStarter(id, title, surface string, lanes []string, adapter string, argv []string, reason string) starterFile {
	return journeyStarterWithCWD(id, title, surface, lanes, adapter, argv, ".", reason)
}

func journeyStarterWithCWD(id, title, surface string, lanes []string, adapter string, argv []string, cwd, reason string) starterFile {
	return starterFile{
		ID:              id,
		RelPath:         filepath.ToSlash(filepath.Join(journeyRootRel, id+".yaml")),
		Reason:          reason,
		Lanes:           append([]string(nil), lanes...),
		ValidateJourney: true,
		Body:            renderJourneyWithCWD(id, title, surface, lanes, adapter, argv, cwd),
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

func jsRunnerArgv(pm, runner string, args ...string) []string {
	var argv []string
	switch pm {
	case "pnpm":
		argv = []string{"pnpm", "exec", runner}
	case "yarn":
		argv = []string{"yarn", runner}
	default:
		argv = []string{"npm", "exec", runner}
	}
	return append(argv, args...)
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

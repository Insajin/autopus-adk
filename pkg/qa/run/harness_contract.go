package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func harnessContract(projectDir string) HarnessContract {
	journeyRoot := filepath.Join(projectDir, ".autopus", "qa", "journeys")
	artifactRoot := filepath.Join(projectDir, ".autopus", "qa", "runs")
	return HarnessContract{
		Role:                 "harness",
		JourneyPackOwnership: "project-local",
		JourneyPackRoot:      filepath.ToSlash(filepath.Clean(journeyRoot)),
		RuntimeArtifactRoot:  filepath.ToSlash(filepath.Clean(artifactRoot)),
		GeneratedPolicy:      "ADK owns adapters, execution, redaction, and feedback; the target project owns concrete Journey Packs.",
		Guidance:             "Create or review project-local Journey Packs before GUI execution; do not hard-code product-specific journeys into ADK.",
	}
}

func projectLocalJourneySetupGaps(opts Options, packs []journey.Pack) []SetupGap {
	if hasGUIExplorePack(packs) || !isGUIExploreRequest(opts) {
		return nil
	}
	return []SetupGap{{
		Adapter:   "gui-explore",
		JourneyID: "project-local-gui-explore",
		Reason:    projectLocalGUIJourneyReason(),
	}}
}

func projectLocalJourneyHints(opts Options, packs []journey.Pack) []SetupGap {
	if hasGUIExplorePack(packs) || isGUIExploreRequest(opts) || !hasDesktopGUISignals(opts.ProjectDir) {
		return nil
	}
	return []SetupGap{{
		Adapter:   "gui-explore",
		JourneyID: "project-local-gui-explore",
		Reason:    "desktop GUI tooling detected; " + projectLocalGUIJourneyReason(),
	}}
}

func hasGUIExplorePack(packs []journey.Pack) bool {
	for _, pack := range packs {
		if pack.Adapter.ID == "gui-explore" && journey.HasLane(pack, "gui-explore") {
			return true
		}
	}
	return false
}

func isGUIExploreRequest(opts Options) bool {
	if opts.AdapterID == "gui-explore" || strings.EqualFold(opts.Lane, "gui-explore") {
		return true
	}
	return false
}

func projectLocalGUIJourneyReason() string {
	return "project-local gui-explore Journey Pack required: ADK is a harness; create .autopus/qa/journeys/<id>.yaml with allowed origins, forbidden actions, deterministic oracles, and redacted artifact retention"
}

func hasDesktopGUISignals(projectDir string) bool {
	if existsAny(projectDir,
		"src-tauri/Cargo.toml",
		"src-tauri/tauri.conf.json",
		"src-tauri/tauri.conf.json5",
		"e2e-tests/wdio.conf.mjs",
		"e2e-macos/run-macos-smoke.mjs",
		"scripts/visual/run-macos-wkwebview-suite.mjs",
		"scripts/visual/run-windows-webview2-suite.mjs",
	) {
		return true
	}
	pkg, ok := readHarnessPackageJSON(filepath.Join(projectDir, "package.json"))
	if !ok {
		return false
	}
	for _, dep := range []string{"@tauri-apps/api", "electron", "@electron/remote"} {
		if pkg.hasDependency(dep) {
			return true
		}
	}
	for script := range pkg.Scripts {
		name := strings.ToLower(script)
		if strings.Contains(name, "tauri") || strings.Contains(name, "appium") ||
			strings.Contains(name, "webdriver") || strings.Contains(name, "visual:macos") ||
			strings.Contains(name, "visual:windows") || strings.Contains(name, "e2e:linux") {
			return true
		}
	}
	return false
}

func existsAny(root string, rels ...string) bool {
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			return true
		}
	}
	return false
}

type harnessPackageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func readHarnessPackageJSON(path string) (harnessPackageJSON, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return harnessPackageJSON{}, false
	}
	var pkg harnessPackageJSON
	if err := json.Unmarshal(body, &pkg); err != nil {
		return harnessPackageJSON{}, false
	}
	return pkg, true
}

func (pkg harnessPackageJSON) hasDependency(name string) bool {
	if _, ok := pkg.Dependencies[name]; ok {
		return true
	}
	_, ok := pkg.DevDependencies[name]
	return ok
}

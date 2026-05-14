package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// HasDesktopGUISignals detects project-local desktop GUI tooling without
// inferring concrete QA journeys from ADK itself.
func HasDesktopGUISignals(projectDir string) bool {
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
	pkg, ok := readPackageJSON(filepath.Join(projectDir, "package.json"))
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

type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func readPackageJSON(path string) (packageJSON, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return packageJSON{}, false
	}
	var pkg packageJSON
	if err := json.Unmarshal(body, &pkg); err != nil {
		return packageJSON{}, false
	}
	return pkg, true
}

func (pkg packageJSON) hasDependency(name string) bool {
	if _, ok := pkg.Dependencies[name]; ok {
		return true
	}
	_, ok := pkg.DevDependencies[name]
	return ok
}

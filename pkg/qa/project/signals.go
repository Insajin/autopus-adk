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

// HasBrowserSignals detects browser-facing project signals without inferring
// concrete staging journeys from ADK itself.
func HasBrowserSignals(projectDir string) bool {
	if existsAny(projectDir,
		"playwright.config.ts",
		"playwright.config.js",
		"playwright.config.mjs",
		"playwright.config.cjs",
		"next.config.ts",
		"next.config.js",
		"nuxt.config.ts",
		"nuxt.config.js",
		"vite.config.ts",
		"vite.config.js",
	) {
		return true
	}
	pkg, ok := readPackageJSON(filepath.Join(projectDir, "package.json"))
	if !ok {
		return false
	}
	for _, dep := range []string{"@playwright/test", "playwright", "next", "nuxt", "vite", "react", "vue", "svelte"} {
		if pkg.hasDependency(dep) {
			return true
		}
	}
	return false
}

// HasAndroidSignals detects project-local Android tooling without inferring
// concrete QA journeys from ADK itself. Bare build.gradle/settings.gradle are
// excluded because they are too broad to imply an Android app.
func HasAndroidSignals(projectDir string) bool {
	return existsAny(projectDir,
		"android/build.gradle",
		"android/build.gradle.kts",
		"android/app/build.gradle",
		"android/app/build.gradle.kts",
		"android/app/src/main/AndroidManifest.xml",
		"app/build.gradle",
		"app/build.gradle.kts",
		"AndroidManifest.xml",
	)
}

// HasIOSSignals detects project-local iOS tooling without inferring concrete QA
// journeys from ADK itself. Bare Info.plist is excluded because it is too broad
// to imply an iOS app.
func HasIOSSignals(projectDir string) bool {
	if existsAny(projectDir,
		"ios/Runner.xcodeproj",
		"ios/Runner.xcworkspace",
		"ios/Podfile",
		"Podfile",
	) {
		return true
	}
	for _, p := range []string{"*.xcodeproj", "*.xcworkspace", "ios/*.xcodeproj", "ios/*.xcworkspace"} {
		if m, _ := filepath.Glob(filepath.Join(projectDir, filepath.FromSlash(p))); len(m) > 0 {
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

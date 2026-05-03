package adapter

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Detection struct {
	AdapterID      string `json:"adapter_id"`
	SetupGapReason string `json:"setup_gap_reason,omitempty"`
}

func Detect(projectDir string) []Detection {
	seen := map[string]Detection{}
	add := func(id string) {
		item := Detection{AdapterID: id}
		if reason := missingReason(id); reason != "" {
			item.SetupGapReason = reason
		}
		seen[id] = item
	}
	if exists(filepath.Join(projectDir, "go.mod")) {
		add("go-test")
	}
	if pkg, ok := readPackageJSON(filepath.Join(projectDir, "package.json")); ok {
		if hasScript(pkg, "test") || len(pkg.Scripts) > 0 {
			add("node-script")
		}
		if hasDep(pkg, "vitest") || exists(filepath.Join(projectDir, "vitest.config.ts")) || exists(filepath.Join(projectDir, "vitest.config.js")) {
			add("vitest")
		}
		if hasDep(pkg, "jest") {
			add("jest")
		}
		if hasDep(pkg, "@playwright/test") || exists(filepath.Join(projectDir, "playwright.config.ts")) || exists(filepath.Join(projectDir, "playwright.config.js")) {
			add("playwright")
		}
	}
	if exists(filepath.Join(projectDir, "pytest.ini")) || fileContains(filepath.Join(projectDir, "pyproject.toml"), "pytest") {
		add("pytest")
	}
	if exists(filepath.Join(projectDir, "Cargo.toml")) {
		add("cargo-test")
	}
	out := make([]Detection, 0, len(seen))
	for _, id := range []string{"go-test", "node-script", "vitest", "jest", "playwright", "pytest", "cargo-test"} {
		if item, ok := seen[id]; ok {
			out = append(out, item)
		}
	}
	return out
}

func WithSetupGaps() []Metadata {
	items := Registry()
	for index, item := range items {
		if reason := missingReason(item.ID); reason != "" {
			items[index].SetupGapReason = reason
		}
	}
	return items
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

func hasScript(pkg packageJSON, name string) bool {
	_, ok := pkg.Scripts[name]
	return ok
}

func hasDep(pkg packageJSON, name string) bool {
	_, ok := pkg.Dependencies[name]
	if ok {
		return true
	}
	_, ok = pkg.DevDependencies[name]
	return ok
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileContains(path, needle string) bool {
	body, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(body), needle)
}

func missingReason(id string) string {
	item, ok := ByID(id)
	if !ok {
		return "unknown adapter"
	}
	for _, binary := range item.RequiredBinaries {
		if _, err := exec.LookPath(binary); err != nil {
			return "missing required binary: " + binary
		}
	}
	if id == "canary-template" {
		return "explicit safe canary command is required"
	}
	return ""
}

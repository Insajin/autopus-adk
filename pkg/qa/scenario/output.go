package scenario

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GeneratedDirName is the leaf directory compiled specs land in. It sits inside
// the project's own Playwright testDir so the existing runner discovers the
// specs with no config change; a directory outside testDir would compile
// successfully and then never run.
const GeneratedDirName = "autopus-generated"

// FixtureRel is the capture fixture 'auto qa init' emits.
var FixtureRel = filepath.Join(".autopus", "qa", "capture", "autopus-capture.fixture.cjs")

// testDirPattern reads a string-literal testDir out of a Playwright config. The
// match is deliberately narrow: a computed testDir is not resolvable without
// executing project code, and the fallback is a documented default rather than
// a guess that silently writes specs somewhere the runner ignores.
var testDirPattern = regexp.MustCompile(`testDir\s*:\s*['"]([^'"\n]{1,120})['"]`)

var playwrightConfigNames = []string{
	"playwright.config.ts", "playwright.config.js",
	"playwright.config.mjs", "playwright.config.cjs",
}

// DefaultTestDir is used when no Playwright config states one. Playwright's own
// default is the config directory, but writing generated specs to the project
// root would scatter them, so the conventional e2e directory is used.
const DefaultTestDir = "e2e"

// TestDir resolves the Playwright testDir for a project, relative to the
// project root, plus the config file it was read from.
func TestDir(projectDir string) (dir string, configRel string) {
	for _, name := range playwrightConfigNames {
		body, err := os.ReadFile(filepath.Join(projectDir, name))
		if err != nil {
			continue
		}
		match := testDirPattern.FindSubmatch(body)
		if match == nil {
			return DefaultTestDir, name
		}
		value := strings.TrimSpace(string(match[1]))
		value = strings.TrimPrefix(value, "./")
		if value == "" || filepath.IsAbs(value) || strings.Contains(value, "..") {
			return DefaultTestDir, name
		}
		return filepath.FromSlash(value), name
	}
	return DefaultTestDir, ""
}

// SpecDir is the directory compiled specs are written to.
func SpecDir(projectDir string) string {
	dir, _ := TestDir(projectDir)
	return filepath.Join(projectDir, dir, GeneratedDirName)
}

// SpecPath is the file one scenario compiles to.
func SpecPath(projectDir, scenarioID string) string {
	return filepath.Join(SpecDir(projectDir), scenarioID+".spec.ts")
}

// FixtureImport computes the specifier the generated spec uses to reach the
// capture fixture. It is relative so the emitted file stays portable across
// checkouts, and slash-separated because that is what module resolution wants
// on every platform.
func FixtureImport(projectDir, specPath string) (string, error) {
	fixture := filepath.Join(projectDir, FixtureRel)
	rel, err := filepath.Rel(filepath.Dir(specPath), fixture)
	if err != nil {
		return "", err
	}
	slashed := filepath.ToSlash(rel)
	if !strings.HasPrefix(slashed, ".") {
		slashed = "./" + slashed
	}
	return slashed, nil
}

// FixtureExists reports whether the capture producer has been scaffolded. A
// compiled spec that imports a missing fixture fails at collection time with a
// module error, which reads as a broken harness rather than a missing step.
func FixtureExists(projectDir string) bool {
	info, err := os.Stat(filepath.Join(projectDir, FixtureRel))
	return err == nil && !info.IsDir()
}

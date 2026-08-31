package scaffold

import (
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// expectedCapturePolicy is the policy both gui-explore examples must declare. The
// desktop and browser examples share one template precisely so they cannot drift.
func expectedCapturePolicy() capture.Policy {
	return capture.Policy{
		Mode:            capture.ModeOnFailure,
		Streams:         []string{capture.StreamScreenshot, capture.StreamConsole, capture.StreamNetwork, capture.StreamTrace},
		Screenshot:      capture.ScreenshotPerStep,
		ConsoleSeverity: capture.SeverityWarning,
		RetainLocal:     true,
		ReplayScript:    capture.ReplayOptional,
	}
}

// TestBrowserGUIExplorePackExampleDeclaresTypedCapturePolicy asserts the browser
// example is a valid gui-explore pack carrying the same capture contract as the
// desktop one, since the capture producer assets are inert without such a pack.
func TestBrowserGUIExplorePackExampleDeclaresTypedCapturePolicy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := browserGUIExplorePackExample(projectSignals{PackageManager: "npm"})

	var pack journey.Pack
	require.NoError(t, yaml.Unmarshal([]byte(body), &pack))
	require.NoError(t, journey.Validate(pack, dir))

	assert.Equal(t, BrowserGUIJourneyID, pack.ID)
	assert.Equal(t, "frontend", pack.Surface)
	assert.Equal(t, "gui-explore", pack.Adapter.ID)
	assert.Equal(t, []string{"gui-explore"}, pack.Lanes)
	assert.Equal(t, expectedCapturePolicy(), pack.GUI.Capture)
	assert.Equal(t, []string{defaultBrowserGUIOrigin}, pack.GUI.AllowedOrigins)
	// Only enforceable labels ship: the guard matches Playwright method names, so a
	// business label like `payment` would advertise a guarantee nothing provides.
	assert.Equal(t, []string{"mutation"}, pack.GUI.ForbiddenActions)
	assert.Equal(t, []string{"src/**", "e2e/**"}, pack.SourceRefs.OwnedPaths)
	assert.Equal(t, "SPEC-QAMESH-003", pack.SourceRefs.SourceSpec)
	// Capture-enabled packs declare no artifacts: the harness emits capture_index
	// itself and the guard receipt witnesses enforcement.
	assert.Empty(t, pack.Artifacts)
	assert.NotContains(t, body, "artifacts:")
}

// TestBrowserGUIExplorePackExampleUsesDetectedOrigin asserts a detected baseURL
// wins over the default and still produces a pack that validates.
func TestBrowserGUIExplorePackExampleUsesDetectedOrigin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := browserGUIExplorePackExample(projectSignals{PackageManager: "npm", BaseOrigin: "http://127.0.0.1:4173"})

	var pack journey.Pack
	require.NoError(t, yaml.Unmarshal([]byte(body), &pack))
	require.NoError(t, journey.Validate(pack, dir))
	assert.Equal(t, []string{"http://127.0.0.1:4173"}, pack.GUI.AllowedOrigins)
}

// TestExploreGrepArgvSeparatesArgsOnlyForNPM pins the one argv detail that decides
// whether a copied example actually selects the read-only subset. npm drops
// `--grep` as an unknown CLI config unless a `--` separator precedes it, while
// `pnpm exec` and Yarn 2+ forward that separator verbatim - Playwright then reads
// `--grep` and `@explore` as positional file filters and runs the whole suite,
// which is the mutation-blocked outcome the pack exists to avoid.
func TestExploreGrepArgvSeparatesArgsOnlyForNPM(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"npm", "exec", "playwright", "test", "--", "--grep", "@explore"}, exploreGrepArgv(""))
	assert.Equal(t, []string{"pnpm", "exec", "playwright", "test", "--grep", "@explore"}, exploreGrepArgv("pnpm"))
	assert.Equal(t, []string{"yarn", "playwright", "test", "--grep", "@explore"}, exploreGrepArgv("yarn"))

	for _, packageManager := range []string{"", "npm", "pnpm", "yarn"} {
		command := journey.Command{Argv: exploreGrepArgv(packageManager), CWD: ".", Timeout: "120s"}
		assert.NoError(t, journey.ValidateCommand("gui-explore", command, nil, t.TempDir(), "qa_journey"), packageManager)
	}
}

// TestDetectJourneyStartersEmitsNoGUIExplorePack is the regression this scope
// reduction exists for. A generated gui-explore pack cannot pass before the project
// owns a read-only exploration subset, and gui-explore is a must lane in the default
// prelaunch profile, so generating one turned the release gate red on projects that
// had configured nothing yet. The capture assets still ship: they are what a
// hand-written pack needs, and the README carries the pack to copy.
func TestDetectJourneyStartersEmitsNoGUIExplorePack(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T) string{
		"browser playwright project": func(t *testing.T) string {
			dir := playwrightProject(t)
			writeFile(t, filepath.Join(dir, "e2e", "login.spec.ts"), "export default {};\n")
			return dir
		},
		"desktop gui project": func(t *testing.T) string {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "src-tauri", "Cargo.toml"), "[package]\nname = \"desktop\"\n")
			writeFile(t, filepath.Join(dir, "package.json"), `{"devDependencies":{"@playwright/test":"^1.0.0"}}`)
			writeFile(t, filepath.Join(dir, "playwright.config.ts"), "export default {}\n")
			return dir
		},
	}
	for name, project := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			starters := detectJourneyStarters(project(t), false)

			assert.Empty(t, guiExploreStarters(starters))
			assert.False(t, hasStarterID(starters, BrowserGUIJourneyID))
			assert.False(t, hasStarterID(starters, DesktopGUIJourneyID))
			for _, id := range []string{"gui-capture-fixture", "gui-capture-reporter", "gui-capture-readme"} {
				assert.True(t, hasStarterID(starters, id), "missing capture asset %q", id)
			}
		})
	}
}

func TestDetectBaseOriginFallsBackWhenConfigIsNotAPlainOrigin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{"absolute origin", "export default { use: { baseURL: 'http://127.0.0.1:4173' } };\n", "http://127.0.0.1:4173"},
		{"https origin", "export default { use: { baseURL: \"https://staging.example.com\" } };\n", "https://staging.example.com"},
		{"trailing slash normalized", "export default { use: { baseURL: 'http://localhost:3000/' } };\n", "http://localhost:3000"},
		{"origin with path", "export default { use: { baseURL: 'http://127.0.0.1:4173/app' } };\n", ""},
		{"origin with query", "export default { use: { baseURL: 'http://127.0.0.1:4173?tenant=a' } };\n", ""},
		{"origin with fragment", "export default { use: { baseURL: 'http://127.0.0.1:4173#top' } };\n", ""},
		{"origin with credentials", "export default { use: { baseURL: 'http://user:pass@127.0.0.1:4173' } };\n", ""},
		{"non literal expression", "export default { use: { baseURL: process.env.BASE_URL } };\n", ""},
		{"relative url", "export default { use: { baseURL: '/app' } };\n", ""},
		{"unsupported scheme", "export default { use: { baseURL: 'file:///tmp/app' } };\n", ""},
		{"no baseURL", "export default { use: {} };\n", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "playwright.config.ts"), testCase.config)
			assert.Equal(t, testCase.want, detectBaseOrigin(dir))
		})
	}
}

func TestDetectBaseOriginReturnsEmptyWithoutConfig(t *testing.T) {
	t.Parallel()
	assert.Empty(t, detectBaseOrigin(t.TempDir()))
}

func guiExploreStarters(starters []starterFile) []starterFile {
	found := make([]starterFile, 0, 1)
	for _, starter := range starters {
		for _, lane := range starter.Lanes {
			if lane == "gui-explore" {
				found = append(found, starter)
				break
			}
		}
	}
	return found
}

func hasStarterID(starters []starterFile, id string) bool {
	for _, starter := range starters {
		if starter.ID == id {
			return true
		}
	}
	return false
}

package scaffold

import (
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const packageNoTestScript = `{"name":"b","private":true,"devDependencies":{"@playwright/test":"^1.58.0"}}`

const packageWithTestScript = `{"name":"b","private":true,"scripts":{"test":"vitest run"}}`

func starterIDs(starters []starterFile) []string {
	ids := make([]string, 0, len(starters))
	for _, starter := range starters {
		ids = append(ids, starter.ID)
	}
	return ids
}

// assertFastStartersValid asserts every emitted starter declares the fast lane,
// requests journey validation, and renders a Journey Pack the loader accepts.
func assertFastStartersValid(t *testing.T, dir string, starters []starterFile) {
	t.Helper()
	require.NotEmpty(t, starters)
	for _, starter := range starters {
		assert.Equal(t, []string{"fast"}, starter.Lanes, starter.ID)
		assert.True(t, starter.ValidateJourney, starter.ID)
		var pack journey.Pack
		require.NoErrorf(t, yaml.Unmarshal([]byte(starter.Body), &pack), "starter %s", starter.ID)
		require.NoErrorf(t, journey.Validate(pack, dir), "starter %s", starter.ID)
		assert.Equal(t, starter.ID, pack.ID)
	}
}

// TestFastStartersGoSurvivesNodePackageWithoutTestScript is the regression: a repo
// with go.mod plus a package.json carrying no runnable test signal used to declare
// no fast-lane pack at all, because package.json overwrote the single detected
// stack and the Node branch then contributed nothing. The `fast` must-lane became
// `setup_gap: missing-journey-pack` and the release gate returned `blocked`.
func TestFastStartersGoSurvivesNodePackageWithoutTestScript(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/b\n\ngo 1.26\n")
	writeFile(t, filepath.Join(dir, "package.json"), packageNoTestScript)

	signals := detectSignals(dir)
	require.Equal(t, []string{"go", "node"}, signals.Stacks)

	starters := fastStarters(signals)
	ids := starterIDs(starters)
	assert.Contains(t, ids, "go-fast")
	// The Node package legitimately contributes nothing here; it must no longer
	// suppress the Go lane.
	assert.Equal(t, []string{"go-fast"}, ids)
	assertFastStartersValid(t, dir, starters)
}

// TestFastStartersPolyglotEmitsBothLanes asserts a Go+Node repo with a runnable
// Node test script declares both fast-lane packs. Multiple packs on one lane are
// supported: run.Execute iterates every selected pack.
func TestFastStartersPolyglotEmitsBothLanes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/b\n\ngo 1.26\n")
	writeFile(t, filepath.Join(dir, "package.json"), packageWithTestScript)

	starters := fastStarters(detectSignals(dir))

	assert.Equal(t, []string{"go-fast", "node-fast"}, starterIDs(starters))
	assertFastStartersValid(t, dir, starters)
}

// TestFastStartersSingleStackUnchanged pins the pre-existing single-stack output so
// the multi-stack rewrite cannot alter repos that were already working.
func TestFastStartersSingleStackUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		files  map[string]string
		wantID string
	}{
		{
			name:   "go only",
			files:  map[string]string{"go.mod": "module example.com/b\n\ngo 1.26\n"},
			wantID: "go-fast",
		},
		{
			name:   "node with test script only",
			files:  map[string]string{"package.json": packageWithTestScript},
			wantID: "node-fast",
		},
		{
			name:   "python only",
			files:  map[string]string{"pytest.ini": "[pytest]\n"},
			wantID: "python-fast",
		},
		{
			name:   "rust only",
			files:  map[string]string{"Cargo.toml": "[package]\nname = \"mylib\"\n"},
			wantID: "rust-fast",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, body := range tc.files {
				writeFile(t, filepath.Join(dir, name), body)
			}
			starters := fastStarters(detectSignals(dir))
			assert.Equal(t, []string{tc.wantID}, starterIDs(starters))
			assertFastStartersValid(t, dir, starters)
		})
	}
}

// TestFastStartersOrderIsDeterministic asserts the emitted order does not depend on
// the order the caller lists stacks in, so repeated `auto qa init` runs declare the
// same packs in the same sequence.
func TestFastStartersOrderIsDeterministic(t *testing.T) {
	t.Parallel()

	signals := projectSignals{
		Stacks:         []string{"rust", "node", "python", "go"},
		PackageManager: "npm",
		Package:        packageManifest{Scripts: map[string]string{"test": "vitest run"}},
	}
	want := []string{"go-fast", "node-fast", "python-fast", "rust-fast"}
	for range 3 {
		assert.Equal(t, want, starterIDs(fastStarters(signals)))
	}
}

// TestFastStarterIDsAreUniquePerStack verifies the no-collision claim rather than
// assuming it: every stack's starter IDs must be disjoint, since all fast-lane
// packs now land in one journeys directory keyed by ID.
func TestFastStarterIDsAreUniquePerStack(t *testing.T) {
	t.Parallel()

	// Each variant a stack can produce, including every Node fallback branch.
	variants := []projectSignals{
		{Stacks: []string{"go"}},
		{Stacks: []string{"python"}},
		{Stacks: []string{"rust"}},
		{Stacks: []string{"node"}, Package: packageManifest{Scripts: map[string]string{"test": "x"}}},
		{Stacks: []string{"node"}, Package: packageManifest{DevDependencies: map[string]string{"vitest": "^2"}}},
		{Stacks: []string{"node"}, Package: packageManifest{DevDependencies: map[string]string{"jest": "^29"}}},
		{Stacks: []string{"node"}, Package: packageManifest{Scripts: map[string]string{"build": "x"}}},
	}
	seen := map[string]string{}
	for _, signals := range variants {
		starters := fastStarters(signals)
		require.Len(t, starters, 1)
		id := starters[0].ID
		stack := signals.Stacks[0]
		if prev, ok := seen[id]; ok {
			assert.Equal(t, stack, prev, "id %s shared across stacks", id)
		}
		seen[id] = stack
	}
	// Node contributes at most one starter per repo, so the four node IDs never
	// collide with each other either.
	assert.Len(t, seen, 7)
}

// TestDominantStackPreservesLegacyPrecedence guards the collateral-damage boundary:
// projectSignals.Stack still drives QA target scoring and CI workflow rendering, so
// it must resolve to exactly the winner the old last-write-wins detection produced
// for every stack combination.
func TestDominantStackPreservesLegacyPrecedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		stacks []string
		want   string
	}{
		{stacks: nil, want: ""},
		{stacks: []string{"go"}, want: "go"},
		{stacks: []string{"node"}, want: "node"},
		{stacks: []string{"python"}, want: "python"},
		{stacks: []string{"rust"}, want: "rust"},
		{stacks: []string{"go", "node"}, want: "node"},
		{stacks: []string{"go", "python"}, want: "go"},
		{stacks: []string{"go", "rust"}, want: "go"},
		{stacks: []string{"node", "python"}, want: "node"},
		{stacks: []string{"node", "rust"}, want: "node"},
		{stacks: []string{"python", "rust"}, want: "python"},
		{stacks: []string{"go", "node", "python", "rust"}, want: "node"},
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.want, dominantStack(tc.stacks), "stacks %v", tc.stacks)
	}
}

// TestDetectJourneyStartersIncludesEveryFastPack asserts the polyglot fast packs
// survive assembly in detectJourneyStarters, not just fastStarters in isolation.
func TestDetectJourneyStartersIncludesEveryFastPack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/b\n\ngo 1.26\n")
	writeFile(t, filepath.Join(dir, "package.json"), packageWithTestScript)

	ids := starterIDs(detectJourneyStarters(dir, false))

	assert.Contains(t, ids, "go-fast")
	assert.Contains(t, ids, "node-fast")
}

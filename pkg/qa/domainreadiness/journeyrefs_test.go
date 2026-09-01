package domainreadiness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeJourneyPack drops a minimal valid Journey Pack into the project so the
// compiler has something real to resolve refs against.
func writeJourneyPack(t *testing.T, projectDir, id, lane string) {
	t.Helper()
	dir := filepath.Join(projectDir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body := "id: " + id + "\n" +
		"title: " + id + "\n" +
		"surface: cli\n" +
		"lanes: [" + lane + "]\n" +
		"adapter:\n  id: go-test\n" +
		"command:\n  argv: [\"go\", \"test\", \"./...\"]\n  cwd: .\n  timeout: 60s\n" +
		"checks:\n  - id: unit\n    type: unit_test\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, id+".yaml"), []byte(body), 0o644))
}

// TestCompileCatalogReportsDanglingJourneyPackRefs is the D20 guard: a catalog
// naming a Journey Pack the project does not have must not compile to valid.
func TestCompileCatalogReportsDanglingJourneyPackRefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scenario := sampleScenario("core-1", "core")
	scenario.JourneyPackRefs = []string{"go-fast", "node-build-fast"}
	writeJourneyPack(t, dir, "go-fast", "fast")

	plan, err := CompileCatalog(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Scenarios:     []Scenario{scenario},
	}, CompileOptions{ProjectDir: dir, Lane: "fast"})
	require.NoError(t, err)

	assert.True(t, plan.Validation.Valid, "structural validation is unaffected by ref resolution")
	assert.False(t, plan.Valid, "a dangling journey_pack_ref must not compile to valid")

	require.Len(t, plan.JourneyRefGaps, 1)
	assert.Equal(t, "core-1", plan.JourneyRefGaps[0].ScenarioID)
	assert.Equal(t, "node-build-fast", plan.JourneyRefGaps[0].JourneyPackRef)
	assert.Equal(t, JourneyRefGapNotFound, plan.JourneyRefGaps[0].Reason)

	require.Len(t, plan.ScenarioPlans, 1)
	assert.Contains(t, plan.ScenarioPlans[0].SetupGaps, JourneyRefGapNotFound+":node-build-fast")
}

// TestCompileCatalogAcceptsResolvableAndAbsentJourneyPackRefs pins both halves of
// the rule: a ref that resolves is clean, and declaring no refs at all is clean
// too. Silence asserts nothing, so a freshly initialized project that owns no
// Journey Packs yet is not reported as defective.
func TestCompileCatalogAcceptsResolvableAndAbsentJourneyPackRefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeJourneyPack(t, dir, "go-fast", "fast")

	resolved := sampleScenario("core-1", "core")
	resolved.JourneyPackRefs = []string{"go-fast"}
	unbound := sampleScenario("core-2", "core")
	unbound.JourneyPackRefs = nil

	plan, err := CompileCatalog(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Scenarios:     []Scenario{resolved, unbound},
	}, CompileOptions{ProjectDir: dir, Lane: "fast"})
	require.NoError(t, err)

	assert.True(t, plan.Valid)
	assert.Empty(t, plan.JourneyRefGaps)
}

// TestCompileCatalogFlagsRefsItCannotVerify asserts an unreadable Journey Pack
// directory is reported as unverified rather than silently passing. Claiming a
// ref is fine when the packs could not be read is the same unearned certainty
// the cross-check exists to remove.
func TestCompileCatalogFlagsRefsItCannotVerify(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packs := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(packs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(packs, "broken.yaml"), []byte("id: [oops\n"), 0o644))

	scenario := sampleScenario("core-1", "core")
	scenario.JourneyPackRefs = []string{"go-fast"}

	plan, err := CompileCatalog(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Scenarios:     []Scenario{scenario},
	}, CompileOptions{ProjectDir: dir, Lane: "fast"})
	require.NoError(t, err)

	assert.False(t, plan.Valid)
	require.Len(t, plan.JourneyRefGaps, 1)
	assert.Equal(t, JourneyRefGapUnverified, plan.JourneyRefGaps[0].Reason)
	assert.NotEmpty(t, plan.JourneyRefGaps[0].Detail)
}

// TestStarterCatalogForProjectBindsOnlyRealJourneyPacks is the other half of
// D20: the shipped starter must not be born with refs nothing can satisfy.
func TestStarterCatalogForProjectBindsOnlyRealJourneyPacks(t *testing.T) {
	t.Parallel()

	// No Journey Packs yet - the shape `auto qa init` writes the catalog in.
	virgin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(virgin, "go.mod"), []byte("module x\n\ngo 1.26\n"), 0o644))

	catalog := StarterCatalogForProject(virgin)
	for _, scenario := range catalog.Scenarios {
		assert.Empty(t, scenario.JourneyPackRefs, "scenario %s must not promise a pack the project lacks", scenario.ScenarioID)
	}
	plan, err := CompileCatalog(catalog, CompileOptions{ProjectDir: virgin, Lane: "full"})
	require.NoError(t, err)
	assert.True(t, plan.Valid)
	assert.Empty(t, plan.JourneyRefGaps)

	// Same project after Journey Packs exist: refs bind to the real pack ids.
	bound := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bound, "go.mod"), []byte("module x\n\ngo 1.26\n"), 0o644))
	writeJourneyPack(t, bound, "go-fast", "fast")

	catalog = StarterCatalogForProject(bound)
	core := findStarterScenario(t, catalog, "project-core-readiness")
	assert.Equal(t, []string{"go-fast"}, core.JourneyPackRefs)

	plan, err = CompileCatalog(catalog, CompileOptions{ProjectDir: bound, Lane: "full"})
	require.NoError(t, err)
	assert.True(t, plan.Valid)
	assert.Empty(t, plan.JourneyRefGaps)
}

func findStarterScenario(t *testing.T, catalog Catalog, id string) Scenario {
	t.Helper()
	for _, scenario := range catalog.Scenarios {
		if scenario.ScenarioID == id {
			return scenario
		}
	}
	t.Fatalf("scenario %q not found in starter catalog", id)
	return Scenario{}
}

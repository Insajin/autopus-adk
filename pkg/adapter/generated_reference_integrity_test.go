package adapter_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/antigravity"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

// danglingReferenceBaseline pins the local path references that generated
// surfaces still make to something no install manifest writes. It is a
// ratchet, not an allowance: a reference outside this set fails the test, and
// a reference that has been fixed must be deleted from the set, so the list
// can only shrink.
//
// Every entry names an ADK source file. A consumer repo installs the harness
// without the ADK checkout, so these references resolve only in this dogfood
// workspace. Each is either wired (install the target) or removed (inline what
// the generated surface needs) as it is worked off.
var danglingReferenceBaseline = map[string]bool{
	"content/profiles/executor/{name}.md":                 true,
	"content/profiles/executor/{profile}.md":              true,
	"content/profiles/executor/{stack}.md":                true,
	"content/rules/deferred-tools.md":                     true,
	"content/rules/spec-quality.md":                       true,
	"content/rules/techstack-freshness.md":                true,
	"content/skills/spec-review.md":                       true,
	"content/workflows/route_a.md":                        true,
	"content/workflows/route_a.schema.json":               true,
	"content/workflows/route_a.{md":                       true,
	"content/workflows/route_team.{md":                    true,
	"pkg/content/ax.go":                                   true,
	"templates/claude/workflows/route_a.workflow.js.tmpl": true,
	"templates/shared/prd-minimal.md.tmpl":                true,
	"templates/shared/prd-standard.md.tmpl":               true,
	"templates/shared/prd-{PRD_MODE}.md.tmpl":             true,
	"templates/shared/scenarios-api.md.tmpl":              true,
	"templates/shared/scenarios-cli.md.tmpl":              true,
	"templates/shared/scenarios-frontend.md.tmpl":         true,
	"templates/shared/scenarios-library.md.tmpl":          true,
}

// danglingReferenceOccurrenceCeiling caps total occurrences of baselined
// references so an existing violation cannot silently multiply across files.
const danglingReferenceOccurrenceCeiling = 121

func generateAllPlatforms(t *testing.T) *installedSurface {
	t.Helper()
	ctx := context.Background()
	cfg := config.DefaultFullConfig("reference-integrity")
	cfg.Platforms = []string{"claude-code", "codex", "antigravity-cli", "opencode", "omp"}

	generators := []func(root string) (*adapter.PlatformFiles, error){
		func(root string) (*adapter.PlatformFiles, error) {
			return claude.NewWithRoot(root).Generate(ctx, cfg)
		},
		func(root string) (*adapter.PlatformFiles, error) {
			return codex.NewWithRoot(root).Generate(ctx, cfg)
		},
		func(root string) (*adapter.PlatformFiles, error) {
			return antigravity.NewWithRoot(root).Generate(ctx, cfg)
		},
		func(root string) (*adapter.PlatformFiles, error) {
			return opencode.NewWithRoot(root).Generate(ctx, cfg)
		},
		func(root string) (*adapter.PlatformFiles, error) {
			return omp.NewWithRoot(root).Generate(ctx, cfg)
		},
	}

	surface := newInstalledSurface()
	for _, generate := range generators {
		pf, err := generate(t.TempDir())
		require.NoError(t, err)
		surface.add(pf.Files)
	}
	return surface
}

// A generated surface that names a path the installer never writes sends the
// agent to a file that does not exist in a consumer repo. This repo hides the
// failure because the ADK checkout sits next to the installed surface, so the
// assertion runs against the generated bodies and the installed path union
// rather than against the working tree.
//
// The earlier codex path-canonicalisation test only asserted that the string
// ".codex/rules/autopus/" had disappeared, which a prefix rewrite satisfies
// while leaving "AGENTS.mdbranding.md" behind. Asserting on the resolved
// target instead of the removed substring is what closes that class.
func TestGeneratedSurfaces_ReferenceOnlyInstalledPaths(t *testing.T) {
	t.Parallel()
	surface := generateAllPlatforms(t)

	observed := map[string]map[string]int{}
	for file, body := range surface.files {
		surface.collectDangling(file, body, observed)
	}

	total := 0
	for _, ref := range sortedRefs(observed) {
		count, examples := refSummary(observed[ref])
		total += count
		if !danglingReferenceBaseline[ref] {
			t.Errorf("generated surface references %q, which no install manifest writes"+
				" (%d occurrences, e.g. %s)", ref, count, examples)
		}
	}

	stale := make([]string, 0, len(danglingReferenceBaseline))
	for ref := range danglingReferenceBaseline {
		if observed[ref] == nil {
			stale = append(stale, ref)
		}
	}
	sort.Strings(stale)
	for _, ref := range stale {
		t.Errorf("baseline entry %q no longer occurs; delete it from danglingReferenceBaseline"+
			" so the ratchet cannot loosen again", ref)
	}

	if total > danglingReferenceOccurrenceCeiling {
		t.Errorf("dangling reference occurrences %d exceed ceiling %d;"+
			" lower the ceiling when you fix references, never raise it",
			total, danglingReferenceOccurrenceCeiling)
	}
}

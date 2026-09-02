package adapter_test

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// componentPathRe pulls the path out of an "Installed Components" bullet. A
// bullet may describe an invocation route instead of a path, which carries no
// path to check.
var componentPathRe = regexp.MustCompile(
	`^- [^:]+: (\.?[A-Za-z0-9_][A-Za-z0-9_./<>*-]*)`)

// unlistedInstalledFamilies pins the manifest path families that Installed
// Components deliberately omits, so a family added later cannot slip in
// unlisted. This is a ratchet: shrink it, never grow it.
var unlistedInstalledFamilies = map[string]string{
	".autopus/claude-code-permissions.json": "derived permission cache, not a component an operator installs",
	".autopus/plugins":                      "runtime plugin staging, rewritten on every run",
	".claude/statusline-combined.sh":        "generated companion of .claude/statusline.sh, which is listed",
	".claude/statusline-user-command.txt":   "generated companion of .claude/statusline.sh, which is listed",
	".claude/workflows":                     "route workflow programs consumed by the Claude runtime, not operator-facing",
	".git/hooks":                            "git-managed hook, reported by `auto doctor` instead",
	".mcp.json":                             "MCP server registry shared with non-harness tooling",
	"opencode.json":                         "OpenCode bootstrap config named in the OpenCode Notes section",
	"AGENTS.md":                             "the document that carries this very list",
}

// installedComponentBullets returns the path in every Installed Components
// bullet of a rendered marker section.
func installedComponentBullets(t *testing.T, section string) []string {
	t.Helper()
	_, after, found := strings.Cut(section, "## Installed Components\n")
	require.True(t, found, "marker section must declare Installed Components")
	body, _, _ := strings.Cut(after, "\n## ")

	var paths []string
	for _, line := range strings.Split(body, "\n") {
		m := componentPathRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		paths = append(paths, strings.TrimSuffix(m[1], "/"))
	}
	require.NotEmpty(t, paths, "Installed Components must list at least one path")
	return paths
}

// family reduces an installed path to the first two segments, which is the
// granularity Installed Components describes.
func family(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + "/" + parts[1]
}

// rootDocOwners enumerates the adapters that can author the AGENTS.md marker
// and the platform set under which each actually owns it. Opencode wins the
// arbitration whenever it is installed (codexOwnsRootDoc returns false and the
// codex adapter drops its AGENTS.md mapping), so the codex marker is only
// reachable without opencode. Each marker is checked against the surface its
// own platform set installs, not a union it never sees.
var rootDocOwners = []struct {
	owner     string
	platforms []string
}{
	{owner: "opencode", platforms: allPlatforms},
	{owner: "codex", platforms: []string{"claude-code", "codex", "antigravity-cli", "omp"}},
}

// Direction 1: every path the list advertises must be installed. Direction 2:
// every installed path family must be advertised. Without direction 2 the
// reported defect passes, because a list naming only Codex and OpenCode is
// internally consistent while Claude, Gemini, and OMP go unmentioned and an
// operator reads the install as half-finished.
func TestRootDoc_InstalledComponentsMatchManifests(t *testing.T) {
	t.Parallel()

	pendingUnlisted := make(map[string]bool, len(unlistedInstalledFamilies))
	for fam := range unlistedInstalledFamilies {
		pendingUnlisted[fam] = true
	}

	for _, owner := range rootDocOwners {
		surface := generateSurface(t, owner.platforms)
		section, ok := surface.files["AGENTS.md"]
		require.True(t, ok, "%s must own an AGENTS.md mapping for %v", owner.owner, owner.platforms)

		installedFamilies := map[string]bool{}
		for p := range surface.files {
			installedFamilies[family(p)] = true
		}

		listed := map[string]bool{}
		for _, p := range installedComponentBullets(t, section) {
			listed[family(p)] = true
			if !surface.resolve(p, "") && !surface.resolve(p+"/", "") {
				t.Errorf("%s marker lists %q, which no install manifest writes", owner.owner, p)
			}
		}

		var missing []string
		for fam := range installedFamilies {
			if listed[fam] {
				continue
			}
			if _, allowed := unlistedInstalledFamilies[fam]; allowed {
				continue
			}
			missing = append(missing, fam)
		}
		sort.Strings(missing)
		for _, fam := range missing {
			t.Errorf("%s marker omits installed family %q from Installed Components", owner.owner, fam)
		}

		for fam := range unlistedInstalledFamilies {
			if !installedFamilies[fam] {
				continue
			}
			delete(pendingUnlisted, fam)
		}
	}

	for fam := range pendingUnlisted {
		t.Errorf("unlistedInstalledFamilies entry %q is no longer installed by any platform;"+
			" delete it so the ratchet cannot loosen again", fam)
	}
}

package adapter_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/antigravity"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

// featureCounts holds per-category file counts for a single platform.
type featureCounts struct {
	Agents int
	Rules  int
	Skills int
}

type platformResult struct {
	name   string
	counts featureCounts
}

// classifyFile categorizes a FileMapping into agents, rules, or skills.
// Returns the category name or empty string if uncategorized.
//
// Rule classification precedes agent classification so stale shared paths such
// as .agents/rules/autopus still land in the rule bucket. Current Codex output
// intentionally has no markdown rule mapping because .codex/rules is an
// execpolicy surface in Codex 0.149.1.
func classifyFile(f adapter.FileMapping) string {
	p := strings.ToLower(f.TargetPath)
	switch {
	case strings.Contains(p, ".agents/plugins/") || strings.Contains(p, ".autopus/plugins/"):
		return ""
	case strings.Contains(p, "skills/") || strings.Contains(p, "skills\\"):
		return "skills"
	case isRuleTargetPath(p):
		return "rules"
	case strings.Contains(p, "agents/") || strings.Contains(p, "agents\\"):
		return "agents"
	default:
		return ""
	}
}

// countFeatures tallies agents, rules, and skills from a PlatformFiles result.
func countFeatures(pf *adapter.PlatformFiles) featureCounts {
	var c featureCounts
	for _, f := range pf.Files {
		switch classifyFile(f) {
		case "agents":
			c.Agents++
		case "rules":
			c.Rules++
		case "skills":
			c.Skills++
		}
	}
	return c
}

// parityPct computes min/max * 100 for a set of counts. Returns 100 if max is 0.
func parityPct(counts ...int) float64 {
	minV, maxV := counts[0], counts[0]
	for _, v := range counts[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if maxV == 0 {
		return 100.0
	}
	return float64(minV) / float64(maxV) * 100.0
}

func TestParity_CrossPlatformFeatures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := config.DefaultFullConfig("parity-test")

	platforms := []struct {
		name     string
		generate func(t *testing.T) *adapter.PlatformFiles
	}{
		{
			name: "claude",
			generate: func(t *testing.T) *adapter.PlatformFiles {
				t.Helper()
				dir := t.TempDir()
				a := claude.NewWithRoot(dir)
				pf, err := a.Generate(ctx, cfg)
				require.NoError(t, err)
				return pf
			},
		},
		{
			name: "codex",
			generate: func(t *testing.T) *adapter.PlatformFiles {
				t.Helper()
				dir := t.TempDir()
				a := codex.NewWithRoot(dir)
				pf, err := a.Generate(ctx, cfg)
				require.NoError(t, err)
				return pf
			},
		},
		{
			name: "gemini",
			generate: func(t *testing.T) *adapter.PlatformFiles {
				t.Helper()
				dir := t.TempDir()
				a := antigravity.NewWithRoot(dir)
				pf, err := a.Generate(ctx, cfg)
				require.NoError(t, err)
				return pf
			},
		},
		{
			name: "omp",
			generate: func(t *testing.T) *adapter.PlatformFiles {
				t.Helper()
				dir := t.TempDir()
				a := omp.NewWithRoot(dir)
				pf, err := a.Generate(ctx, cfg)
				require.NoError(t, err)
				return pf
			},
		},
	}

	results := make([]platformResult, len(platforms))
	for i, p := range platforms {
		pf := p.generate(t)
		results[i] = platformResult{name: p.name, counts: countFeatures(pf)}
	}

	// Print parity report table
	t.Log("\n=== Parity Report ===")
	t.Logf("%-10s %8s %8s %8s", "Platform", "Agents", "Rules", "Skills")
	t.Logf("%-10s %8s %8s %8s", "--------", "------", "-----", "------")
	for _, r := range results {
		t.Logf("%-10s %8d %8d %8d",
			r.name, r.counts.Agents, r.counts.Rules, r.counts.Skills)
	}

	claudeCounts := countsForPlatform(t, results, "claude")
	codexCounts := countsForPlatform(t, results, "codex")
	geminiCounts := countsForPlatform(t, results, "gemini")

	codexAgentParity := parityPct(claudeCounts.Agents, codexCounts.Agents)
	codexRulesParity := parityPct(claudeCounts.Rules, codexCounts.Rules)
	codexSkillsParity := parityPct(claudeCounts.Skills, codexCounts.Skills)
	geminiRulesParity := parityPct(claudeCounts.Rules, geminiCounts.Rules)
	overallSkillsParity := parityPct(claudeCounts.Skills, codexCounts.Skills, geminiCounts.Skills)

	t.Logf("\n%-10s %7.1f%% %7.1f%% %7.1f%%",
		"Codex", codexAgentParity, codexRulesParity, codexSkillsParity)
	t.Logf("%-10s %7s %7.1f%% %7.1f%%",
		"Gemini", "-", geminiRulesParity, overallSkillsParity)

	// P0 gate: Codex keeps native agent parity. Its markdown-rule count is
	// intentionally zero because policy moved to native skills and AGENTS.md.
	assert.GreaterOrEqualf(t, codexAgentParity, 95.0,
		"P0 FAIL: Codex agent parity %.1f%% < 95%%", codexAgentParity)
	assert.Zero(t, codexCounts.Rules,
		"Codex 0.149.1 must not receive inert repository markdown rules")

	// omp joins the report as of SPEC-OMP-001 REQ-012. Its rule count is pinned
	// against claude rather than a second magic constant, and it is what proves
	// the classification of the relocated rule surface holds on real generated
	// output: .omp/rules/autopus-<name>.md has to satisfy isRuleTargetPath, and
	// extractRuleName has to strip the autopus- prefix, or omp silently reports
	// a rule count that no longer matches the 14 content/rules sources.
	ompCounts := countsForPlatform(t, results, "omp")
	assert.Equal(t, claudeCounts.Rules, ompCounts.Rules,
		"omp must report the same rule count as claude")

	// Skills parity is informational (not gated) but still logged
	if codexSkillsParity < 95.0 {
		t.Logf("INFO: Codex skills parity %.1f%% < 95%% (not gated)", codexSkillsParity)
	}
	if geminiRulesParity < 95.0 {
		t.Logf("INFO: Gemini rules parity %.1f%% < 95%% (not gated in this test)", geminiRulesParity)
	}
}

func countsForPlatform(t *testing.T, results []platformResult, name string) featureCounts {
	t.Helper()
	for _, result := range results {
		if result.name == name {
			return result.counts
		}
	}
	t.Fatalf("platform result not found: %s", name)
	return featureCounts{}
}

func TestParity_ClassifyFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{".claude/agents/autopus/planner.md", "agents"},
		{".codex/agents/planner.toml", "agents"},
		{".gemini/agents/autopus/planner.md", "agents"},
		{".claude/rules/autopus/branding.md", "rules"},
		{".gemini/rules/branding.md", "rules"},
		// omp namespaces rules by file-name prefix inside .omp/rules/, the one
		// directory it scans, and that scan is non-recursive (SPEC-OMP-001
		// REQ-002).
		{".omp/rules/autopus-branding.md", "rules"},
		// The pre-relocation omp path matched the agent substring too. No
		// adapter emits it now, but a stale manifest can still name it and it is
		// the only case that pins the rule-before-agent ordering.
		{".agents/rules/autopus/branding.md", "rules"},
		{".omp/agents/planner.md", "agents"},
		// A relocated hook-fired body is still a rule (REQ-CONDRULE-VERIFY-02);
		// the compiled manifest beside it is not.
		{".claude/hooks/autopus/conditional/lore-commit.md", "rules"},
		{".claude/hooks/autopus/conditional-rules.json", ""},
		{".claude/skills/auto/SKILL.md", "skills"},
		{".codex/skills/codex-auto-skill/SKILL.md", "skills"},
		{".agents/skills/auto/SKILL.md", "skills"},
		{".agents/plugins/marketplace.json", ""},
		{".autopus/plugins/auto/skills/auto/SKILL.md", ""},
		{"CLAUDE.md", ""},
		{"AGENTS.md", ""},
		{".mcp.json", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := classifyFile(adapter.FileMapping{TargetPath: tt.path})
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParity_ExtractRuleName pins generated rule paths to source names.
func TestParity_ExtractRuleName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{".claude/rules/autopus/branding.md", "branding.md"},
		{".gemini/rules/branding.md", "branding.md"},
		{".omp/rules/autopus-branding.md", "branding.md"},
		{".omp/rules/autopus-lore-commit.md", "lore-commit.md"},
		{filepath.FromSlash(".omp/rules/autopus-worktree-safety.md"), "worktree-safety.md"},
		// A user's own file in the shared .omp/rules directory carries no
		// prefix, so its name passes through untouched.
		{".omp/rules/mine.md", "mine.md"},
		{".claude/hooks/autopus/conditional/lore-commit.md", "lore-commit.md"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, extractRuleName(tt.path))
		})
	}
}

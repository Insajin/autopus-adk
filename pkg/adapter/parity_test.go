package adapter_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
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
func classifyFile(f adapter.FileMapping) string {
	p := strings.ToLower(f.TargetPath)
	switch {
	case strings.Contains(p, ".agents/plugins/") || strings.Contains(p, ".autopus/plugins/"):
		return ""
	case strings.Contains(p, "skills/") || strings.Contains(p, "skills\\"):
		return "skills"
	case strings.Contains(p, "agents/") || strings.Contains(p, "agents\\"):
		return "agents"
	case strings.Contains(p, "rules/") || strings.Contains(p, "rules\\") ||
		strings.Contains(p, "rules-autopus"):
		return "rules"
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
				a := gemini.NewWithRoot(dir)
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

	// P0 gate for this rollout: Codex must remain aligned with Claude on
	// managed agents and rules. Other platforms are reported but not gated here.
	assert.GreaterOrEqualf(t, codexAgentParity, 95.0,
		"P0 FAIL: Codex agent parity %.1f%% < 95%%", codexAgentParity)
	assert.GreaterOrEqualf(t, codexRulesParity, 95.0,
		"P0 FAIL: Codex rules parity %.1f%% < 95%%", codexRulesParity)

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
		{".codex/rules-autopus-branding.md", "rules"},
		{".gemini/rules/branding.md", "rules"},
		{".claude/skills/auto/SKILL.md", "skills"},
		{".codex/skills/auto-skill.md", "skills"},
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

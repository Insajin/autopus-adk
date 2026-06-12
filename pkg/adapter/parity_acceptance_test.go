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
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

// generatePlatformRules runs the named adapter's Generate in a temp dir and
// returns a map of rule basename -> rule file content for inspection.
func generatePlatformRules(t *testing.T, platform string) map[string]string {
	t.Helper()
	ctx := context.Background()
	cfg := config.DefaultFullConfig("parity-test")
	dir := t.TempDir()

	var pf *adapter.PlatformFiles
	var err error
	switch platform {
	case "claude":
		pf, err = claude.NewWithRoot(dir).Generate(ctx, cfg)
	case "codex":
		pf, err = codex.NewWithRoot(dir).Generate(ctx, cfg)
	case "gemini":
		pf, err = gemini.NewWithRoot(dir).Generate(ctx, cfg)
	case "opencode":
		pf, err = opencode.NewWithRoot(dir).Generate(ctx, cfg)
	default:
		t.Fatalf("unknown platform: %s", platform)
	}
	require.NoError(t, err)

	rules := make(map[string]string)
	for _, f := range pf.Files {
		pathLower := strings.ToLower(f.TargetPath)
		isRule := strings.Contains(pathLower, "rules/") || strings.Contains(pathLower, "rules\\") || strings.Contains(pathLower, "rules-autopus")
		if isRule {
			rules[extractRuleName(f.TargetPath)] = string(f.Content)
		}
	}
	return rules
}

// S1: Gemini가 누락 규칙 3종을 실제 내용과 함께 생성 (Must, REQ-001).
func TestAcceptance_S1_GeminiMissingRulesPresentWithContent(t *testing.T) {
	rules := generatePlatformRules(t, "gemini")

	deferred, ok := rules["deferred-tools.md"]
	require.True(t, ok, "deferred-tools.md must be generated")
	assert.Contains(t, deferred, "# Deferred Tools Loading")
	assert.Contains(t, deferred, "Antigravity CLI")

	identity, ok := rules["project-identity.md"]
	require.True(t, ok, "project-identity.md must be generated")
	assert.Contains(t, identity, "# Project Identity")

	quality, ok := rules["spec-quality.md"]
	require.True(t, ok, "spec-quality.md must be generated")
	assert.Contains(t, quality, "SPEC Quality Checklist")
}

// S2: Gemini 규칙 집합이 content 소스 집합과 정확히 일치 (Must, REQ-001, REQ-002).
func TestAcceptance_S2_GeminiRuleSetMatchesSource(t *testing.T) {
	rules := generatePlatformRules(t, "gemini")
	got := make([]string, 0, len(rules))
	for name := range rules {
		got = append(got, name)
	}
	want := []string{
		"branding.md", "context7-docs.md", "deferred-tools.md", "doc-storage.md",
		"file-size-limit.md", "language-policy.md", "lore-commit.md",
		"objective-reasoning.md", "project-identity.md", "shell-portability.md",
		"spec-quality.md", "subagent-delegation.md", "techstack-freshness.md",
		"worktree-safety.md",
	}
	// @AX:NOTE: [AUTO] magic constant — 14 equals the total canonical rule count in content/rules/; update this oracle when source rules are added or removed
	assert.ElementsMatch(t, want, got, "Gemini rule basenames must equal the 14 content/rules sources")
	assert.Len(t, got, 14)
	// Gemini rule exclusion set must be empty.
	assert.Empty(t, platformRuleExclusions["gemini"])
}

// S5: platform frontmatter 값이 어댑터 식별자와 일치 (Should, REQ-003).
func TestAcceptance_S5_PlatformFrontmatterValues(t *testing.T) {
	for name, content := range generatePlatformRules(t, "gemini") {
		if val, ok := parsePlatformFromFrontmatter(content); ok {
			assert.Equal(t, "antigravity-cli", val, "gemini rule %s platform value", name)
		}
	}
	for name, content := range generatePlatformRules(t, "codex") {
		if val, ok := parsePlatformFromFrontmatter(content); ok {
			assert.Equal(t, "codex", val, "codex rule %s platform value", name)
		}
	}

	// Gate reports exactly 0 platform-value mismatch findings.
	findings, err := runCoverageGate(context.Background(), t.TempDir(), config.DefaultFullConfig("parity-test"),
		[]string{"claude", "codex", "gemini", "opencode"}, platformRuleExclusions, platformSkillExclusions, nil)
	require.NoError(t, err)
	mismatches := 0
	for _, f := range findings {
		if f.Type == "platform-value" {
			mismatches++
		}
	}
	assert.Equal(t, 0, mismatches, "platform-value mismatch findings must be 0")
}

// S6: 기존 플랫폼 출력 후방호환 유지 (Must, REQ-005).
func TestAcceptance_S6_ExistingPlatformsBackwardCompatible(t *testing.T) {
	codexRules := generatePlatformRules(t, "codex")
	assert.Len(t, codexRules, 14, "codex generates exactly 14 rules")
	codexPlatformCount := 0
	for _, content := range codexRules {
		if val, ok := parsePlatformFromFrontmatter(content); ok {
			assert.Equal(t, "codex", val)
			codexPlatformCount++
		}
	}
	// @AX:NOTE: [AUTO] magic constant — 8 of 14 codex rules carry platform: codex frontmatter; update when codex rule set changes
	assert.Equal(t, 8, codexPlatformCount, "8 codex rules retain platform: codex frontmatter")

	assert.Len(t, generatePlatformRules(t, "claude"), 14, "claude generates exactly 14 rules")
	assert.Len(t, generatePlatformRules(t, "opencode"), 14, "opencode generates exactly 14 rules")
}

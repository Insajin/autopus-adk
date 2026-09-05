package adapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/antigravity"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

// goldenFile holds the pre-change emitted-rule checksums captured before
// SPEC-CONDRULE-001 landed. It is the S8 oracle and must never be re-baselined.
const goldenFile = "testdata/rules_golden_prechange.json"

// alwaysRuleFiles are the rules that stay unconditional on every platform.
//
// Issue #185 removed four of the original ten: context7-docs, doc-storage,
// spec-quality, and techstack-freshness became skill-scoped, so their
// frontmatter gained the skillScoped key and the pre-change digests no longer
// describe the emitted file. They are not dropped from verification — their
// text is pinned against the embedded source below, which is a stronger oracle
// than a digest because it cannot be satisfied by re-baselining.
var alwaysRuleFiles = map[string]bool{
	"branding.md":            true,
	"deferred-tools.md":      true,
	"language-policy.md":     true,
	"objective-reasoning.md": true,
	"project-identity.md":    true,
	"subagent-delegation.md": true,
}

// skillScopedRuleFiles are the four rules issue #185 reclassified.
var skillScopedRuleFiles = []string{
	"context7-docs.md",
	"doc-storage.md",
	"spec-quality.md",
	"techstack-freshness.md",
}

// generatePlatform writes one platform surface into a fresh root.
func generatePlatform(t *testing.T, platform string) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	cfg := config.DefaultFullConfig("golden")

	var err error
	switch platform {
	case "claude":
		_, err = claude.NewWithRoot(dir).Generate(ctx, cfg)
	case "gemini":
		_, err = antigravity.NewWithRoot(dir).Generate(ctx, cfg)
	case "opencode":
		_, err = opencode.NewWithRoot(dir).Generate(ctx, cfg)
	default:
		t.Fatalf("unknown platform %q", platform)
	}
	require.NoError(t, err)
	return dir
}

func loadGolden(t *testing.T) map[string]map[string]string {
	t.Helper()
	raw, err := os.ReadFile(goldenFile)
	require.NoError(t, err)
	var golden map[string]map[string]string
	require.NoError(t, json.Unmarshal(raw, &golden))
	return golden
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "generated file must exist: %s", path)
	return digestOf(data)
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// stickyRuleFiles are the two rules REQ-STICKYRULE-MAP-01 marks sticky. They are
// the only two of the ten unconditional rules whose emitted bytes this SPEC
// changes, and the change is exactly one added frontmatter line.
var stickyRuleFiles = map[string]bool{
	"language-policy.md":     true,
	"objective-reasoning.md": true,
}

// digestWithoutStickyKey removes the single `alwaysApply: true` frontmatter line
// and digests what remains.
//
// This is what lets the pre-change goldens stay untouched for the two sticky
// rules instead of being re-baselined to whatever the generator now emits. A
// re-baselined digest would accept any drift that happened to land in the same
// commit; comparing the stripped bytes accepts one specific difference — the
// sticky key — and nothing else, so REQ-STICKYRULE-SCHEMA-02's "verbatim" claim
// is verified rather than assumed.
func digestWithoutStickyKey(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "generated file must exist: %s", path)

	content := string(data)
	require.Equal(t, 1, strings.Count(content, stickyKeyLine),
		"%s must carry the sticky key exactly once", path)
	require.Contains(t, frontmatterBlock(t, content), stickyKeyLine,
		"%s must confine the sticky key to frontmatter", path)

	return digestOf([]byte(strings.Replace(content, stickyKeyLine+"\n", "", 1)))
}

// TestRules_UnconditionalRulesStayByteIdentical is the S8 oracle for
// REQ-CONDRULE-SCHEMA-02 and INV-005, extended by SPEC-STICKYRULE-001's
// REQ-STICKYRULE-SCHEMA-02.
//
// Characterization test: the historical goldens are not re-baselined. The
// Claude deferred-tools rule is version-sensitive and has a dedicated latest
// CLI contract; Codex is excluded because current Codex uses native skills and
// AGENTS.md rather than inert repository markdown rules. Every other active
// unconditional rule remains byte-pinned.
func TestRules_UnconditionalRulesStayByteIdentical(t *testing.T) {
	t.Parallel()

	golden := loadGolden(t)
	require.Len(t, golden, 4, "historical goldens include the retired Codex rule surface")

	checked, sticky := 0, 0
	for _, platform := range []string{"claude", "gemini", "opencode"} {
		paths := golden[platform]
		require.NotEmpty(t, paths, "no goldens for %s", platform)

		dir := generatePlatform(t, platform)
		for target, want := range paths {
			base := filepath.Base(target)
			if !alwaysRuleFiles[base] {
				continue
			}
			if platform == "claude" && base == "deferred-tools.md" {
				continue
			}
			emitted := filepath.Join(dir, filepath.FromSlash(target))

			got := fileDigest(t, emitted)
			if stickyRuleFiles[base] {
				got = digestWithoutStickyKey(t, emitted)
				sticky++
			}
			assert.Equal(t, want, got, "%s emitted content changed for %s", platform, target)
			checked++
		}
	}
	assert.Equal(t, 23, checked, "version-stable unconditional rule emissions checked")
	assert.Equal(t, 8, sticky,
		"the 2 sticky rules across active surfaces, gemini emitting 2 each")
}

// TestRules_NoHookEntryReferencesUnconditionalRules completes S8: no rule that
// carries no runtime trigger may be wired into a hook command. Issue #185 adds
// the skill-scoped four, whose whole point is that nothing fires them.
func TestRules_NoHookEntryReferencesUnconditionalRules(t *testing.T) {
	t.Parallel()

	dir := generatePlatform(t, "claude")
	raw, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)

	settings := string(raw)
	for name := range alwaysRuleFiles {
		assert.NotContains(t, settings, strings.TrimSuffix(name, ".md"),
			"%s must not be referenced by a hook entry", name)
	}
	for _, name := range skillScopedRuleFiles {
		assert.NotContains(t, settings, strings.TrimSuffix(name, ".md"),
			"%s is skill-scoped and must earn no dispatcher entry", name)
	}
}

// TestRules_SkillScopedRulesKeepTheirTextOffClaude replaces the byte-pin the
// four reclassified rules lost when their frontmatter gained the skillScoped
// key. Only claude-code changes where their text lives; every other platform
// keeps emitting the source rule verbatim, so the emitted bytes are compared
// against the embedded source rather than against a digest that a re-baseline
// could quietly satisfy.
func TestRules_SkillScopedRulesKeepTheirTextOffClaude(t *testing.T) {
	opencodeFiles := stickyGenerate(t, "opencode")
	geminiFiles := stickyGenerate(t, "gemini")

	for _, rule := range skillScopedRuleFiles {
		source := stickyContentSource(t, rule)

		assert.Equal(t, pkgcontent.ReplacePlatformReferences(source, "opencode"),
			stickyRuleEmission(t, opencodeFiles, "opencode", rule),
			"opencode emission for %s must be the source bytes, skillScoped key included", rule)

		_, body := stickySplitFrontmatter(t, source)
		assert.Contains(t, stickyRuleEmission(t, geminiFiles, "gemini", rule), body,
			"gemini must inline the untouched content body for %s", rule)
	}
}

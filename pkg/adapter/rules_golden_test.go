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

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

// goldenFile holds the pre-change emitted-rule checksums captured before
// SPEC-CONDRULE-001 landed. It is the S8 oracle and must never be re-baselined.
const goldenFile = "testdata/rules_golden_prechange.json"

// alwaysRuleFiles are the ten rules SPEC-CONDRULE-001 leaves unconditional.
var alwaysRuleFiles = map[string]bool{
	"branding.md":            true,
	"context7-docs.md":       true,
	"deferred-tools.md":      true,
	"doc-storage.md":         true,
	"language-policy.md":     true,
	"objective-reasoning.md": true,
	"project-identity.md":    true,
	"spec-quality.md":        true,
	"subagent-delegation.md": true,
	"techstack-freshness.md": true,
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
	case "codex":
		_, err = codex.NewWithRoot(dir).Generate(ctx, cfg)
	case "gemini":
		_, err = gemini.NewWithRoot(dir).Generate(ctx, cfg)
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
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestRules_UnconditionalRulesStayByteIdentical is the S8 oracle for
// REQ-CONDRULE-SCHEMA-02 and INV-005.
//
// Characterization test: the goldens were captured before the change, so this
// test is green both before and after. It exists to catch collateral drift on
// the ten rules that keep no trigger field.
func TestRules_UnconditionalRulesStayByteIdentical(t *testing.T) {
	t.Parallel()

	golden := loadGolden(t)
	require.Len(t, golden, 4, "goldens cover claude, codex, gemini, opencode")

	checked := 0
	for _, platform := range []string{"claude", "codex", "gemini", "opencode"} {
		paths := golden[platform]
		require.NotEmpty(t, paths, "no goldens for %s", platform)

		dir := generatePlatform(t, platform)
		for target, want := range paths {
			if !alwaysRuleFiles[filepath.Base(target)] {
				continue
			}
			got := fileDigest(t, filepath.Join(dir, filepath.FromSlash(target)))
			assert.Equal(t, want, got, "%s emitted content changed for %s", platform, target)
			checked++
		}
	}
	assert.Equal(t, 50, checked, "10 unconditional rules across 4 platform surfaces")
}

// TestRules_NoHookEntryReferencesUnconditionalRules completes S8: none of the
// ten rules may be wired into a hook command.
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
}

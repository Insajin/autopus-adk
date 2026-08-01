package adapter_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentfs "github.com/insajin/autopus-adk/content"
)

// triggerFieldKeys are the omp-contract fields that must round-trip unchanged.
var triggerFieldKeys = []string{"condition", "scope", "interruptMode", "astCondition"}

// frontmatterBlock returns the YAML frontmatter of a markdown document.
func frontmatterBlock(t *testing.T, raw string) string {
	t.Helper()
	require.True(t, strings.HasPrefix(raw, "---\n"), "document must open with frontmatter")
	rest := raw[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	require.GreaterOrEqual(t, end, 0, "frontmatter must be closed")
	return rest[:end]
}

// triggerLines maps each trigger key to its verbatim source line.
func triggerLines(t *testing.T, frontmatter string) map[string]string {
	t.Helper()
	lines := map[string]string{}
	for _, line := range strings.Split(frontmatter, "\n") {
		for _, key := range triggerFieldKeys {
			if strings.HasPrefix(strings.TrimSpace(line), key+":") {
				lines[key] = strings.TrimRight(line, "\r")
			}
		}
	}
	return lines
}

func sourceRuleFrontmatter(t *testing.T, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(contentfs.FS, "rules/"+name)
	require.NoError(t, err)
	return frontmatterBlock(t, string(raw))
}

// TestRules_TriggerFieldsSurviveCodexAndOpencode is the S7 oracle for
// REQ-CONDRULE-SCHEMA-04 and REQ-CONDRULE-COMPILE-05.
func TestRules_TriggerFieldsSurviveCodexAndOpencode(t *testing.T) {
	t.Parallel()

	sourceLines := triggerLines(t, sourceRuleFrontmatter(t, "lore-commit.md"))
	for _, key := range triggerFieldKeys {
		require.Contains(t, sourceLines, key,
			"content/rules/lore-commit.md must declare %s", key)
	}

	platforms := []struct {
		name        string
		target      string
		extraAssert func(t *testing.T, frontmatter string)
	}{
		{
			name:   "codex",
			target: ".codex/rules/autopus/lore-commit.md",
			extraAssert: func(t *testing.T, frontmatter string) {
				assert.Contains(t, frontmatter, "platform: codex")
			},
		},
		{
			name:        "opencode",
			target:      ".opencode/rules/autopus/lore-commit.md",
			extraAssert: func(t *testing.T, frontmatter string) {},
		},
	}

	for _, platform := range platforms {
		t.Run(platform.name, func(t *testing.T) {
			t.Parallel()
			dir := generatePlatform(t, platform.name)
			raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(platform.target)))
			require.NoError(t, err, "%s must emit %s", platform.name, platform.target)

			frontmatter := frontmatterBlock(t, string(raw))
			for _, key := range triggerFieldKeys {
				assert.Contains(t, frontmatter, sourceLines[key],
					"%s must preserve the %s line verbatim", platform.name, key)
			}
			platform.extraAssert(t, frontmatter)
		})
	}
}

// TestRules_GeminiOutputUnchangedForTriggeredRules completes S7: the gemini
// adapter keeps its pre-change bytes for the rules that gain triggers.
func TestRules_GeminiOutputUnchangedForTriggeredRules(t *testing.T) {
	t.Parallel()

	golden := loadGolden(t)["gemini"]
	require.NotEmpty(t, golden)

	dir := generatePlatform(t, "gemini")
	checked := 0
	for target, want := range golden {
		base := filepath.Base(target)
		if base != "lore-commit.md" && base != "shell-portability.md" && base != "worktree-safety.md" {
			continue
		}
		assert.Equal(t, want, fileDigest(t, filepath.Join(dir, filepath.FromSlash(target))),
			"gemini output changed for %s", target)
		checked++
	}
	assert.Positive(t, checked, "gemini goldens must cover the triggered rules")
}

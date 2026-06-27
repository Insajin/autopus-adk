package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexAdapter_Generate_MinimalityDisciplineRenderedSurfaces(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := codex.NewWithRoot(dir).Generate(context.Background(), config.DefaultFullConfig("minimality-project"))
	require.NoError(t, err)

	reviewTokens := []string{
		"Correctness/Security Findings",
		"Complexity Findings",
		"delete",
		"stdlib",
		"native",
		"yagni",
		"shrink",
		"existing-helper",
		"existing-dependency",
	}
	cases := []struct {
		path   string
		tokens []string
	}{
		{filepath.Join(".agents", "skills", "auto-plan", "SKILL.md"), []string{"Minimality Decision Matrix", "new dependency", "new abstraction", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".agents", "skills", "auto-go", "SKILL.md"), []string{"minimality ladder", "existing code/helper/pattern", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".agents", "skills", "auto-fix", "SKILL.md"), []string{"caller", "shared root-cause", "revise-target", "receipt"}},
		{filepath.Join(".agents", "skills", "auto-review", "SKILL.md"), append(reviewTokens, "receipt")},
		{filepath.Join(".codex", "skills", "auto-plan.md"), []string{"Minimality Decision Matrix", "new dependency", "new abstraction", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".codex", "skills", "auto-go.md"), []string{"minimality ladder", "existing code/helper/pattern", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".codex", "skills", "auto-fix.md"), []string{"caller", "shared root-cause", "revise-target", "receipt"}},
		{filepath.Join(".codex", "skills", "auto-review.md"), append(reviewTokens, "receipt")},
		{filepath.Join(".codex", "prompts", "auto-plan.md"), []string{"Minimality Decision Matrix", "new dependency", "new abstraction", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".codex", "prompts", "auto-go.md"), []string{"minimality ladder", "existing code/helper/pattern", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".codex", "prompts", "auto-fix.md"), []string{"caller", "shared root-cause", "revise-target", "receipt"}},
		{filepath.Join(".codex", "prompts", "auto-review.md"), append(reviewTokens, "receipt")},
		{filepath.Join(".codex", "skills", "agent-pipeline.md"), []string{"minimality ladder", "existing code/helper/pattern", "minimum sufficient verification", "receipt"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			content := readGeneratedCodexSurface(t, dir, tc.path)
			for _, token := range tc.tokens {
				assert.Contains(t, content, token, "%s should contain %q", tc.path, token)
			}
			assert.NotContains(t, content, "Ponytail mode")
			assert.NotContains(t, content, "lean mode")
		})
	}
}

func readGeneratedCodexSurface(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err, rel)
	return string(body)
}

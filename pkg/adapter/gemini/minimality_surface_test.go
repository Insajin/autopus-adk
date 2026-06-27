package gemini

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiAdapter_Generate_MinimalityDisciplineRenderedSurfaces(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := NewWithRoot(dir).Generate(context.Background(), config.DefaultFullConfig("minimality-project"))
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
		{filepath.Join(".gemini", "skills", "autopus", "auto-plan", "SKILL.md"), []string{"Minimality Decision Matrix", "new dependency", "new abstraction", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".gemini", "skills", "autopus", "auto-go", "SKILL.md"), []string{"minimality ladder", "existing code/helper/pattern", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".gemini", "skills", "autopus", "auto-fix", "SKILL.md"), []string{"caller", "shared root-cause", "revise-target", "receipt"}},
		{filepath.Join(".gemini", "skills", "autopus", "auto-review", "SKILL.md"), append(reviewTokens, "receipt")},
		{filepath.Join(".gemini", "skills", "autopus", "agent-pipeline", "SKILL.md"), []string{"minimality ladder", "existing code/helper/pattern", "minimum sufficient verification", "receipt"}},
		{filepath.Join(".gemini", "skills", "auto", "SKILL.md"), []string{"Minimality Decision Matrix", "minimality ladder", "shared root-cause", "Correctness/Security Findings"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			content := readGeneratedGeminiSurface(t, dir, tc.path)
			for _, token := range tc.tokens {
				assert.Contains(t, content, token, "%s should contain %q", tc.path, token)
			}
		})
	}
}

func readGeneratedGeminiSurface(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err, rel)
	return string(body)
}

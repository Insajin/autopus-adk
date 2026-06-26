package adapter_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestE2EInitMakeInterfacesFeelBetterSkill_AllPlatforms(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		generate func(context.Context, string) error
		path     string
	}{
		{
			name: "claude-code",
			generate: func(ctx context.Context, dir string) error {
				cfg := config.DefaultFullConfig("polish-claude")
				cfg.Platforms = []string{"claude-code"}
				_, err := claude.NewWithRoot(dir).Generate(ctx, cfg)
				return err
			},
			path: filepath.Join(".claude", "skills", "autopus", "make-interfaces-feel-better.md"),
		},
		{
			name: "codex",
			generate: func(ctx context.Context, dir string) error {
				cfg := config.DefaultFullConfig("polish-codex")
				cfg.Platforms = []string{"codex"}
				_, err := codex.NewWithRoot(dir).Generate(ctx, cfg)
				return err
			},
			path: filepath.Join(".codex", "skills", "make-interfaces-feel-better.md"),
		},
		{
			name: "gemini",
			generate: func(ctx context.Context, dir string) error {
				cfg := config.DefaultFullConfig("polish-gemini")
				cfg.Platforms = []string{"gemini-cli"}
				_, err := gemini.NewWithRoot(dir).Generate(ctx, cfg)
				return err
			},
			path: filepath.Join(".gemini", "skills", "autopus", "make-interfaces-feel-better", "SKILL.md"),
		},
		{
			name: "opencode",
			generate: func(ctx context.Context, dir string) error {
				cfg := config.DefaultFullConfig("polish-opencode")
				cfg.Platforms = []string{"opencode"}
				_, err := opencode.NewWithRoot(dir).Generate(ctx, cfg)
				return err
			},
			path: filepath.Join(".agents", "skills", "make-interfaces-feel-better", "SKILL.md"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			require.NoError(t, tc.generate(context.Background(), dir))

			data, err := os.ReadFile(filepath.Join(dir, tc.path))
			require.NoError(t, err, "expected generated skill at %s", tc.path)
			assert.Contains(t, string(data), "## Detail Pass")
			assert.Contains(t, string(data), "Do not use `transition: all`")
		})
	}
}

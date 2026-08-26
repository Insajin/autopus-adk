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
	"github.com/insajin/autopus-adk/pkg/config"
)

// TestParity_ClassifyFileCountsConditionalBodies is the S16 oracle for
// REQ-CONDRULE-VERIFY-02.
func TestParity_ClassifyFileCountsConditionalBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{".claude/hooks/autopus/conditional/lore-commit.md", "rules"},
		{".claude/hooks/autopus/conditional/shell-portability.md", "rules"},
		{".claude/hooks/autopus/conditional/worktree-safety.md", "rules"},
		{".claude/hooks/autopus/conditional-rules.json", ""},
		{".claude/hooks/autopus/task-created-validate.sh", ""},
		{".claude/rules/autopus/branding.md", "rules"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, classifyFile(adapter.FileMapping{TargetPath: tt.path}))
		})
	}
}

// TestParity_RelocationConservesRuleCount is the S16 count-conservation oracle
// for REQ-CONDRULE-VERIFY-01.
func TestParity_RelocationConservesRuleCount(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := config.DefaultFullConfig("parity-conditional")

	claudeFiles, err := claude.NewWithRoot(t.TempDir()).Generate(ctx, cfg)
	require.NoError(t, err)
	codexFiles, err := codex.NewWithRoot(t.TempDir()).Generate(ctx, cfg)
	require.NoError(t, err)
	geminiFiles, err := antigravity.NewWithRoot(t.TempDir()).Generate(ctx, cfg)
	require.NoError(t, err)

	claudeCounts := countFeatures(claudeFiles)
	assert.Equal(t, 14, claudeCounts.Rules, "relocation must conserve the claude rule count")
	assert.Zero(t, countFeatures(codexFiles).Rules,
		"Codex 0.149.1 must not receive inert markdown under .codex/rules")
	assert.Equal(t, 14, countFeatures(geminiFiles).Rules)

	var baseline, relocated int
	for _, file := range claudeFiles.Files {
		target := filepath.ToSlash(file.TargetPath)
		switch {
		case strings.HasPrefix(target, ".claude/rules/autopus/"):
			baseline++
		case strings.Contains(target, "hooks/autopus/conditional/"):
			relocated++
		}
	}
	assert.Equal(t, 11, baseline, "eleven rules remain in baseline context")
	assert.Equal(t, 3, relocated, "three rule bodies move under the conditional body root")
}

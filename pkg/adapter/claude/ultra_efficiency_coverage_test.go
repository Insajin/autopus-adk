package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUltraEfficiencyCoverage_PreparedContentErrorsStayExplicit(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())

	_, err := a.prepareContentFiles("missing-content", ".claude/missing")
	assert.ErrorContains(t, err, "컨텐츠 디렉터리 읽기 실패")
	_, err = a.prepareNamedContentFiles("hooks", ".claude/hooks", []string{"missing-hook.sh"})
	assert.ErrorContains(t, err, "컨텐츠 파일 읽기 실패")
}

func TestUltraEfficiencyCoverage_WorkflowExtractionRejectsIncompleteMarkers(t *testing.T) {
	t.Parallel()
	_, err := extractClaudeWorkflowSection("body", "## missing", "")
	assert.ErrorContains(t, err, "시작 marker")
	_, err = extractClaudeWorkflowSection("## start\nbody", "## start", "## missing")
	assert.ErrorContains(t, err, "종료 marker")
}

func TestUltraEfficiencyCoverage_GenerateReportsBlockedManagedTargets(t *testing.T) {
	tests := []struct {
		name      string
		blockPath string
		blockDir  bool
	}{
		{name: "router directory", blockPath: ".claude/skills/auto"},
		{name: "MCP file", blockPath: ".mcp.json", blockDir: true},
		{name: "statusline file", blockPath: ".claude/statusline.sh", blockDir: true},
		{name: "settings file", blockPath: ".claude/settings.json", blockDir: true},
		{name: "file size rule", blockPath: ".claude/rules/autopus/file-size-limit.md", blockDir: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(test.blockPath))
			if test.blockDir {
				blockPathWithDir(t, path)
			} else {
				blockPathWithFile(t, path)
			}
			_, err := NewWithRoot(root).Generate(
				context.Background(), config.DefaultFullConfig("coverage-project"),
			)
			assert.Error(t, err)
		})
	}
}

func blockPathWithFile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("blocked"), 0o644))
}

func blockPathWithDir(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0o755))
}

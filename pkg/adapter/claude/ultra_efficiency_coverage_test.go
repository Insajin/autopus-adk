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

func TestUltraEfficiencyCoverage_EmbeddedContentErrorsStayExplicit(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	cfg := config.DefaultFullConfig("coverage-project")

	_, err := a.copyContentFiles(cfg, "missing-content", ".claude/missing")
	assert.ErrorContains(t, err, "컨텐츠 디렉터리 읽기 실패")
	_, err = a.prepareContentFiles("missing-content", ".claude/missing")
	assert.ErrorContains(t, err, "컨텐츠 디렉터리 읽기 실패")
	_, err = a.copyNamedContentFiles("hooks", ".claude/hooks", []string{"missing-hook.sh"})
	assert.ErrorContains(t, err, "컨텐츠 파일 읽기 실패")
	_, err = a.prepareNamedContentFiles("hooks", ".claude/hooks", []string{"missing-hook.sh"})
	assert.ErrorContains(t, err, "컨텐츠 파일 읽기 실패")
}

func TestUltraEfficiencyCoverage_ContentCopyRejectsBlockedDestinationParent(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "root-is-file")
	blockPathWithFile(t, root)
	a := NewWithRoot(root)
	cfg := config.DefaultFullConfig("coverage-project")

	_, err := a.copyContentFiles(cfg, "rules", "nested/rules")
	assert.ErrorContains(t, err, "대상 디렉터리 생성 실패")
	_, err = a.copyNamedContentFiles("hooks", "nested/hooks", []string{"task-created-validate.sh"})
	assert.ErrorContains(t, err, "대상 디렉터리 생성 실패")
}

func TestUltraEfficiencyCoverage_ContentCopyRejectsDirectoryAtFileTarget(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("coverage-project")

	ruleRoot := t.TempDir()
	ruleTarget := filepath.Join(ruleRoot, "output", "branding.md")
	blockPathWithDir(t, ruleTarget)
	_, err := NewWithRoot(ruleRoot).copyContentFiles(cfg, "rules", "output")
	assert.ErrorContains(t, err, "컨텐츠 파일 쓰기 실패")

	hookRoot := t.TempDir()
	hookTarget := filepath.Join(hookRoot, "output", "task-created-validate.sh")
	blockPathWithDir(t, hookTarget)
	_, err = NewWithRoot(hookRoot).copyNamedContentFiles(
		"hooks", "output", []string{"task-created-validate.sh"},
	)
	assert.ErrorContains(t, err, "컨텐츠 파일 쓰기 실패")
}

func TestUltraEfficiencyCoverage_WorkflowExtractionRejectsIncompleteMarkers(t *testing.T) {
	t.Parallel()
	_, err := extractClaudeWorkflowSection("body", "## missing", "")
	assert.ErrorContains(t, err, "시작 marker")
	_, err = extractClaudeWorkflowSection("## start\nbody", "## start", "## missing")
	assert.ErrorContains(t, err, "종료 marker")
}

func TestUltraEfficiencyCoverage_RouterInstallRejectsBlockedTargets(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("coverage-project")

	dirRoot := t.TempDir()
	blockPathWithFile(t, filepath.Join(dirRoot, ".claude", "skills", "auto"))
	_, err := NewWithRoot(dirRoot).renderRouterCommand(cfg)
	assert.ErrorContains(t, err, "라우터 스킬 디렉터리 생성 실패")

	fileRoot := t.TempDir()
	blockPathWithDir(t, filepath.Join(fileRoot, ".claude", "skills", "auto", "SKILL.md"))
	_, err = NewWithRoot(fileRoot).renderRouterCommand(cfg)
	assert.ErrorContains(t, err, "라우터 스킬 쓰기 실패")
}

func TestUltraEfficiencyCoverage_DetailedWorkflowInstallRejectsBlockedTargets(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("coverage-project")

	dirRoot := t.TempDir()
	blockPathWithFile(t, filepath.Join(dirRoot, ".claude", "skills", "autopus"))
	_, err := NewWithRoot(dirRoot).renderWorkflowSkills(cfg)
	assert.ErrorContains(t, err, "상세 workflow 디렉터리 생성 실패")

	fileRoot := t.TempDir()
	blockPathWithDir(t, filepath.Join(fileRoot, ".claude", "skills", "autopus", "auto-setup.md"))
	_, err = NewWithRoot(fileRoot).renderWorkflowSkills(cfg)
	assert.ErrorContains(t, err, "상세 workflow 쓰기 실패")
}

func TestUltraEfficiencyCoverage_GenerateReportsBlockedManagedTargets(t *testing.T) {
	tests := []struct {
		name      string
		blockPath string
		blockDir  bool
		want      string
	}{
		{name: "router directory", blockPath: ".claude/skills/auto", want: "커맨드 템플릿 렌더링 실패"},
		{name: "MCP file", blockPath: ".mcp.json", blockDir: true, want: ".mcp.json 쓰기 실패"},
		{name: "statusline file", blockPath: ".claude/statusline.sh", blockDir: true, want: "statusline 복사 실패"},
		{name: "settings file", blockPath: ".claude/settings.json", blockDir: true, want: "settings.json"},
		{name: "file size rule", blockPath: ".claude/rules/autopus/file-size-limit.md", blockDir: true, want: "file-size-limit.md 쓰기 실패"},
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
			assert.ErrorContains(t, err, test.want)
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

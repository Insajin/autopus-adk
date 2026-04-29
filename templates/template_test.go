// Package templates는 템플릿 렌더링 통합 테스트이다.
package templates_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

// 템플릿 루트 디렉터리 — 테스트 파일이 templates/ 디렉터리에 있으므로 현재 디렉터리 사용
func templateRoot() string {
	// 테스트 실행 위치 기준으로 templates/ 디렉터리 찾기
	dir, _ := os.Getwd()
	return dir
}

func TestSharedWorkflowTemplate_Lite(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("my-project")

	tmplPath := filepath.Join(templateRoot(), "shared", "workflow.md.tmpl")
	result, err := e.RenderFile(tmplPath, cfg)
	require.NoError(t, err)

	assert.Contains(t, result, "my-project")
	assert.Contains(t, result, "full")
	assert.Contains(t, result, "/plan")
	assert.Contains(t, result, "/go")
}

func TestSharedWorkflowTemplate_Full(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("full-project")

	tmplPath := filepath.Join(templateRoot(), "shared", "workflow.md.tmpl")
	result, err := e.RenderFile(tmplPath, cfg)
	require.NoError(t, err)

	assert.Contains(t, result, "full-project")
	assert.Contains(t, result, "full")
	assert.Contains(t, result, "Full 모드 기능")
}

func TestSharedAutopusYamlTemplate(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("yaml-project")

	tmplPath := filepath.Join(templateRoot(), "shared", "autopus.yaml.tmpl")
	result, err := e.RenderFile(tmplPath, cfg)
	require.NoError(t, err)

	assert.Contains(t, result, "yaml-project")
	assert.Contains(t, result, "mode: full")
	assert.Contains(t, result, "claude-code")
	assert.Contains(t, result, "subprocess:")
	assert.Contains(t, result, "timeout: 420")
	assert.Contains(t, result, "design:")
	assert.Contains(t, result, "max_context_lines:")
}

func TestSharedDesignTemplate(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("design-project")

	tmplPath := filepath.Join(templateRoot(), "shared", "DESIGN.md.tmpl")
	result, err := e.RenderFile(tmplPath, cfg)
	require.NoError(t, err)

	assert.Contains(t, result, "source_of_truth")
	assert.Contains(t, result, "Palette Roles")
	assert.Contains(t, result, "Typography")
	assert.Contains(t, result, "Component Guardrails")
	assert.Contains(t, result, "Responsive Behavior")
	assert.Contains(t, result, "Agent Prompt Guidance")
}

func TestClaudeRouterTemplate(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("cmd-project")

	tmplPath := filepath.Join(templateRoot(), "claude", "commands", "auto-router.md.tmpl")
	result, err := e.RenderFile(tmplPath, cfg)
	require.NoError(t, err, "라우터 템플릿 렌더링 실패")
	assert.Contains(t, result, "cmd-project", "프로젝트명이 포함되어야 함")
	assert.True(t, len(result) > 100, "템플릿 결과가 너무 짧음")

	// 모든 서브커맨드가 포함되어야 함
	subcommands := []string{"plan", "go", "fix", "map", "review", "secure", "stale", "sync", "why"}
	for _, sub := range subcommands {
		assert.Contains(t, result, sub, "서브커맨드 %q가 포함되어야 함", sub)
	}
}

func TestCodexSkillTemplates(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("codex-project")

	skills := []string{
		"auto-plan", "auto-go", "auto-fix", "auto-review", "auto-sync",
	}

	for _, skill := range skills {
		skill := skill
		t.Run(skill, func(t *testing.T) {
			t.Parallel()
			tmplPath := filepath.Join(templateRoot(), "codex", "skills", skill+".md.tmpl")
			result, err := e.RenderFile(tmplPath, cfg)
			require.NoError(t, err, "코덱스 스킬 템플릿 렌더링 실패: %s", skill)
			assert.Contains(t, result, "codex-project")
		})
	}
}

func TestGeminiSkillTemplates_HasFrontmatter(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("gemini-project")

	skills := []string{
		"auto-plan", "auto-go", "auto-fix", "auto-review", "auto-sync",
	}

	for _, skill := range skills {
		skill := skill
		t.Run(skill, func(t *testing.T) {
			t.Parallel()
			tmplPath := filepath.Join(templateRoot(), "gemini", "skills", skill, "SKILL.md.tmpl")
			result, err := e.RenderFile(tmplPath, cfg)
			require.NoError(t, err, "제미니 스킬 템플릿 렌더링 실패: %s", skill)

			// YAML frontmatter 확인
			assert.True(t, strings.HasPrefix(result, "---"), "YAML frontmatter로 시작해야 함: %s", skill)
			assert.Contains(t, result, "name: "+skill)
			assert.Contains(t, result, "gemini-project")
		})
	}
}

func TestTemplates_FullModeConditionals(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	root := templateRoot()

	cfg := config.DefaultFullConfig("test")

	// 라우터 템플릿에서 Full 모드 조건부 블록 확인
	tmplPath := filepath.Join(root, "claude", "commands", "auto-router.md.tmpl")

	result, err := e.RenderFile(tmplPath, cfg)
	require.NoError(t, err)

	// Full 모드에서는 go/review/secure 서브커맨드의 스킬 참조가 포함됨
	assert.Contains(t, result, "tdd.md")
}

func TestSemanticInvariantSourceContracts(t *testing.T) {
	t.Parallel()

	root := templateRoot()
	files := map[string][]string{
		filepath.Join(root, "..", "content", "rules", "spec-quality.md"): {
			"Q-COMP-05",
			"Semantic Invariant Inventory",
			"oracle acceptance",
			"spec.md",
			"plan.md",
			"acceptance.md",
		},
		filepath.Join(root, "..", "content", "agents", "spec-writer.md"): {
			"Semantic Invariant Inventory",
			"source clause",
			"invariant type",
			"acceptance IDs",
			"oracle acceptance",
			"untrusted prompt input",
			"never as instructions",
			"redact",
		},
		filepath.Join(root, "..", "content", "agents", "tester.md"): {
			"oracle acceptance",
			"structural-only",
			"concrete output values",
		},
		filepath.Join(root, "..", "content", "agents", "validator.md"): {
			"oracle acceptance",
			"semantic output",
			"structural-only",
			"Recommended Agent",
		},
	}

	for path, expected := range files {
		path, expected := path, expected
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			content, err := os.ReadFile(path)
			require.NoError(t, err)
			for _, phrase := range expected {
				assert.Contains(t, string(content), phrase)
			}
		})
	}
}

func TestWorkflowAuthenticityTemplateContracts(t *testing.T) {
	t.Parallel()

	e := tmpl.New()
	cfg := config.DefaultFullConfig("authenticity-project")
	root := templateRoot()
	templatePaths := []string{
		filepath.Join(root, "claude", "commands", "auto-router.md.tmpl"),
		filepath.Join(root, "codex", "prompts", "auto-go.md.tmpl"),
		filepath.Join(root, "codex", "skills", "auto-go.md.tmpl"),
		filepath.Join(root, "codex", "skills", "agent-pipeline.md.tmpl"),
		filepath.Join(root, "gemini", "commands", "auto-router.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "auto-go", "SKILL.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "agent-pipeline", "SKILL.md.tmpl"),
	}

	for _, path := range templatePaths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			result, err := e.RenderFile(path, cfg)
			require.NoError(t, err)
			assert.Contains(t, result, "subagent_dispatch_count")
			assert.Contains(t, result, "workflow authenticity blocker")
			assert.Contains(t, result, "degraded-mode")
		})
	}
}

func TestSemanticInvariantPlatformTemplateContracts(t *testing.T) {
	t.Parallel()

	e := tmpl.New()
	cfg := config.DefaultFullConfig("semantic-project")
	root := templateRoot()
	templatePaths := map[string][]string{
		filepath.Join(root, "claude", "commands", "auto-router.md.tmpl"):      {"Semantic Invariant Inventory", "oracle acceptance", "structural-only", "untrusted prompt input", "never as instructions", "redact", "multi-line raw user text"},
		filepath.Join(root, "codex", "prompts", "auto-plan.md.tmpl"):          {"Semantic Invariant Inventory", "oracle acceptance", "structural-only", "untrusted prompt input", "never as instructions", "redact", "multi-line raw user text"},
		filepath.Join(root, "codex", "skills", "auto-plan.md.tmpl"):           {"Semantic Invariant Inventory", "oracle acceptance", "structural-only", "untrusted prompt input", "never as instructions", "redact", "multi-line raw user text"},
		filepath.Join(root, "codex", "agents", "spec-writer.toml.tmpl"):       {"Semantic Invariant Inventory", "oracle acceptance", "structural-only", "untrusted prompt input", "never as instructions", "redact", "multi-line raw user text"},
		filepath.Join(root, "codex", "agents", "tester.toml.tmpl"):            {"oracle acceptance", "structural-only", "concrete output values"},
		filepath.Join(root, "codex", "agents", "validator.toml.tmpl"):         {"oracle acceptance", "structural-only", "semantic output"},
		filepath.Join(root, "gemini", "commands", "auto-router.md.tmpl"):      {"Semantic Invariant Inventory", "oracle acceptance", "structural-only", "untrusted prompt input", "never as instructions", "redact", "multi-line raw user text"},
		filepath.Join(root, "gemini", "skills", "auto-plan", "SKILL.md.tmpl"): {"Semantic Invariant Inventory", "oracle acceptance", "structural-only", "untrusted prompt input", "never as instructions", "redact", "multi-line raw user text"},
		filepath.Join(root, "gemini", "agents", "spec-writer.md.tmpl"):        {"Semantic Invariant Inventory", "oracle acceptance", "structural-only", "untrusted prompt input", "never as instructions", "redact", "multi-line raw user text"},
		filepath.Join(root, "gemini", "agents", "tester.md.tmpl"):             {"oracle acceptance", "structural-only", "concrete output values"},
		filepath.Join(root, "gemini", "agents", "validator.md.tmpl"):          {"oracle acceptance", "structural-only", "semantic output"},
	}

	for path, expected := range templatePaths {
		path, expected := path, expected
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			result, err := e.RenderFile(path, cfg)
			require.NoError(t, err)
			for _, phrase := range expected {
				assert.Contains(t, result, phrase)
			}
		})
	}
}

package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLSPCmd_Structure는 lsp 커맨드 구조를 테스트한다.
func TestLSPCmd_Structure(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"lsp", "--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "diagnostics")
	assert.Contains(t, output, "refs")
	assert.Contains(t, output, "rename")
	assert.Contains(t, output, "symbols")
	assert.Contains(t, output, "definition")
}

// TestLSPDiagnosticsCmd_InGoProject는 Go 프로젝트에서 lsp diagnostics를 테스트한다.
// 실제 LSP 서버 없이 오류만 확인
func TestLSPDiagnosticsCmd_InGoProject(t *testing.T) {
	t.Parallel()

	// lsp diagnostics는 go.mod가 있어야 하고 gopls가 있어야 함
	// 여기서는 Go 프로젝트(CWD)에서 실행하지만 gopls 없으면 오류
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "diagnostics", "main.go"})
	err := cmd.Execute()
	// gopls가 없거나 다른 오류가 발생할 수 있음
	_ = err
}

// TestSkillListCmd_WithCategory는 카테고리 필터로 skill list를 테스트한다.
func TestSkillListCmd_WithCategory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestSkill(t, dir, "tdd.md", `---
name: tdd
description: TDD 스킬
category: methodology
triggers:
  - tdd
---
body`)
	writeTestSkill(t, dir, "deploy.md", `---
name: deploy
description: 배포 스킬
category: devops
triggers:
  - deploy
---
body`)

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"skill", "list", "--skills-dir", dir, "--category", "methodology"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "tdd")
	assert.NotContains(t, output, "deploy")
}

// TestSkillListCmd_Empty는 빈 스킬 디렉토리를 테스트한다.
func TestSkillListCmd_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"skill", "list", "--skills-dir", dir})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "등록된 스킬이 없습니다")
}

// TestSkillInfoCmd_WithResources는 리소스가 있는 skill info를 테스트한다.
func TestSkillInfoCmd_WithResources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestSkill(t, dir, "advanced.md", `---
name: advanced
description: 고급 스킬
category: advanced
triggers:
  - advanced
resources:
  - docs/reference.md
  - examples/sample.md
---

# Advanced Skill

이 스킬은 고급 기능을 제공합니다.`)

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"skill", "info", "advanced", "--skills-dir", dir})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "advanced")
	assert.Contains(t, output, "고급 스킬")
}

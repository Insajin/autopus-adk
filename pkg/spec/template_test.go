package spec_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestScaffold_CreatesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := spec.Scaffold(dir, "AUTH-001", "사용자 인증")
	require.NoError(t, err)

	// 생성된 디렉터리 확인
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-AUTH-001")
	assert.DirExists(t, specDir)

	// 생성된 파일 확인
	assert.FileExists(t, filepath.Join(specDir, "spec.md"))
	assert.FileExists(t, filepath.Join(specDir, "plan.md"))
	assert.FileExists(t, filepath.Join(specDir, "acceptance.md"))
}

func TestScaffold_SpecMdContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := spec.Scaffold(dir, "AUTH-001", "사용자 인증")
	require.NoError(t, err)

	specFile := filepath.Join(dir, ".autopus", "specs", "SPEC-AUTH-001", "spec.md")
	content, err := os.ReadFile(specFile)
	require.NoError(t, err)

	// 필수 섹션 확인
	assert.Contains(t, string(content), "SPEC-AUTH-001")
	assert.Contains(t, string(content), "사용자 인증")
}

func TestScaffold_AlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "DUP-001", "중복 테스트"))
	// 두 번 호출해도 오류 없어야 함 (idempotent)
	err := spec.Scaffold(dir, "DUP-001", "중복 테스트")
	assert.NoError(t, err)
}

func TestLoad_ParsesSpecDocument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "LOAD-001", "로드 테스트"))

	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-LOAD-001")
	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	require.NotNil(t, doc)

	assert.Equal(t, "SPEC-LOAD-001", doc.ID)
}

func TestLoad_ParsesStatusFromFrontmatter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "STATUS-001", "상태 파싱 테스트"))

	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-STATUS-001")
	require.NoError(t, spec.UpdateStatus(specDir, "approved"))

	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	assert.Equal(t, "approved", doc.Status)
}

func TestUpdateStatus_RewritesFrontmatterStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "STATUS-002", "상태 업데이트 테스트"))

	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-STATUS-002")
	require.NoError(t, spec.UpdateStatus(specDir, "approved"))

	content, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	require.NoError(t, err)
	body := string(content)
	assert.Contains(t, body, "status: approved")
	assert.Contains(t, body, "version: 0.1.0")
}

func TestLoad_ParsesAcceptanceMdFromScaffold(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "LOAD-ACC-001", "로드 인수 기준 테스트"))

	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-LOAD-ACC-001")
	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	require.NotNil(t, doc)

	require.Len(t, doc.AcceptanceCriteria, 3)
	assert.Equal(t, "AC-001", doc.AcceptanceCriteria[0].ID)
	assert.Equal(t, "[시나리오 제목]", doc.AcceptanceCriteria[0].Description)
	assert.Equal(t, "[에지 케이스]", doc.AcceptanceCriteria[2].Description)
}

func TestLoad_NonExistentDir(t *testing.T) {
	t.Parallel()

	_, err := spec.Load("/nonexistent/spec/dir")
	assert.Error(t, err)
}

func TestScaffold_Creates4Files(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := spec.Scaffold(dir, "QUAL-001", "품질 개선")
	require.NoError(t, err)

	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-QUAL-001")

	// 4개 파일이 모두 생성되어야 함
	assert.FileExists(t, filepath.Join(specDir, "spec.md"))
	assert.FileExists(t, filepath.Join(specDir, "plan.md"))
	assert.FileExists(t, filepath.Join(specDir, "acceptance.md"))
	assert.FileExists(t, filepath.Join(specDir, "research.md"))
}

func TestScaffold_SpecMdHasStructuredSections(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := spec.Scaffold(dir, "STRUCT-001", "구조화 테스트")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".autopus", "specs", "SPEC-STRUCT-001", "spec.md"))
	require.NoError(t, err)
	body := string(content)

	// YAML 프론트매터 섹션 확인
	assert.Contains(t, body, "id: SPEC-STRUCT-001")
	assert.Contains(t, body, "version: 0.1.0")
	assert.Contains(t, body, "status: draft")

	// 필수 섹션 확인
	assert.Contains(t, body, "## Purpose")
	assert.Contains(t, body, "## Background")
	assert.Contains(t, body, "## Requirements")
	assert.Contains(t, body, "## Acceptance Criteria")
	assert.Contains(t, body, "## Out of Scope")
	assert.Contains(t, body, "## Traceability")
}

func TestScaffold_PlanMdHasStructuredSections(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := spec.Scaffold(dir, "PLAN-001", "플랜 구조 테스트")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".autopus", "specs", "SPEC-PLAN-001", "plan.md"))
	require.NoError(t, err)
	body := string(content)

	// 구조화된 섹션 확인
	assert.Contains(t, body, "## File Impact Analysis")
	assert.Contains(t, body, "## Architecture Considerations")
	assert.Contains(t, body, "## Risks & Mitigations")
	assert.Contains(t, body, "## Exit Criteria")
}

func TestScaffold_AcceptanceMdHasGherkin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := spec.Scaffold(dir, "ACC-001", "인수 테스트")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".autopus", "specs", "SPEC-ACC-001", "acceptance.md"))
	require.NoError(t, err)
	body := string(content)

	// Gherkin 형식 확인
	assert.Contains(t, body, "Given ")
	assert.Contains(t, body, "When ")
	assert.Contains(t, body, "Then ")

	// 필수 섹션 확인
	assert.Contains(t, body, "## Test Scenarios")
	assert.Contains(t, body, "## Edge Cases")
	assert.Contains(t, body, "## Definition of Done")
}

func TestScaffold_ResearchMdHasSections(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := spec.Scaffold(dir, "RES-001", "리서치 테스트")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".autopus", "specs", "SPEC-RES-001", "research.md"))
	require.NoError(t, err)
	body := string(content)

	// 필수 섹션 확인
	assert.Contains(t, body, "## Codebase Analysis")
	assert.Contains(t, body, "## Lore Decisions")
	assert.Contains(t, body, "## Architecture Compliance")
	assert.Contains(t, body, "## Key Findings")
	assert.Contains(t, body, "## Recommendations")
}

func TestLoad_CustomSpecFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specDir := filepath.Join(dir, "myspec")
	require.NoError(t, os.MkdirAll(specDir, 0o755))

	content := `# SPEC-CUSTOM-001: 커스텀 SPEC

## Purpose

커스텀 목적입니다.

## Requirements

시스템은 SHALL 동작합니다.

## Acceptance Criteria

- 기준 1
`
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o644))

	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	assert.Equal(t, "SPEC-CUSTOM-001", doc.ID)
	assert.Equal(t, "커스텀 SPEC", doc.Title)
}

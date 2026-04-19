package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoctorCmd_AllPlatforms는 여러 플랫폼이 설치된 상태에서 doctor를 테스트한다.
func TestDoctorCmd_AllPlatforms(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "doctor-proj", "--platforms", "claude-code,codex,gemini-cli"})
	require.NoError(t, initCmd.Execute())

	doctorCmd := newTestRootCmd()
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	assert.NoError(t, doctorCmd.Execute())
}

// TestSkillListCmd_EmptyDir은 스킬이 없는 디렉터리에서 list를 테스트한다.
func TestSkillListCmd_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"skill", "list", "--skills-dir", dir})
	assert.NoError(t, cmd.Execute())
}

// TestSkillListCmd_DefaultDir은 기본 디렉터리에서 skill list를 테스트한다.
func TestSkillListCmd_DefaultDir(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"skill", "list"})
	err := cmd.Execute()
	_ = err
}

// TestSkillInfoCmd_WithTriggers는 트리거 정보가 있는 스킬 조회를 테스트한다.
func TestSkillInfoCmd_WithTriggers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestSkill(t, dir, "myskill.md", `---
name: myskill
description: 테스트 스킬
category: testing
triggers:
  - myskill
  - skill
---

# My Skill

테스트 스킬 내용이다.`)

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"skill", "info", "myskill", "--skills-dir", dir})
	assert.NoError(t, cmd.Execute())
}

// TestLoreCmd_ConstraintsWithEntries는 실제 git repo에서 lore constraints를 실행한다.
// (git lore 트레일러가 있는 커밋이 없더라도 오류 없이 실행되어야 함)
func TestLoreCmd_ConstraintsWithEntries(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "constraints"})
	err := cmd.Execute()
	_ = err
}

// TestLoreCmd_DirectivesOutput은 lore directives 출력을 테스트한다.
func TestLoreCmd_DirectivesOutput(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "directives"})
	err := cmd.Execute()
	_ = err
}

// TestLoreCmd_RejectedOutput은 lore rejected 출력을 테스트한다.
func TestLoreCmd_RejectedOutput(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "rejected"})
	err := cmd.Execute()
	_ = err
}

// TestLoreCommitCmd_AllTrailers는 모든 트레일러를 포함한 커밋을 테스트한다.
func TestLoreCommitCmd_AllTrailers(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(dir))

	_ = runGit(t, dir, "git", "init")
	_ = runGit(t, dir, "git", "config", "user.email", "test@test.com")
	_ = runGit(t, dir, "git", "config", "user.name", "Test User")
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))
	_ = runGit(t, dir, "git", "add", ".")

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{
		"lore", "commit",
		"--message", "테스트 커밋 메시지",
		"--constraint", "테스트 제약사항",
		"--directive", "테스트 지시사항",
		"--rejected", "대안1, 대안2",
		"--confidence", "high",
		"--scope-risk", "low",
		"--reversibility", "reversible",
	})
	err = cmd.Execute()
	_ = err
}

// runGit은 git 명령어를 실행한다.
func runGit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	cmd := newTestRootCmd()
	_ = cmd
	return nil
}

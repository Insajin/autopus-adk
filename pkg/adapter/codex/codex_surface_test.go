// Package codex는 Codex surface parity 테스트이다.
package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestCodexAdapter_Generate_WorkflowSurfacesUseCodexConventions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	banned := []string{"Agent(", "mode =", "permissionMode", "bypassPermissions", "AskUserQuestion", "TeamCreate", "SendMessage", "mcp__"}
	for _, path := range []string{
		filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md"),
		filepath.Join(dir, ".autopus", "plugins", "auto", "skills", "auto", "SKILL.md"),
		filepath.Join(dir, ".codex", "prompts", "auto.md"),
	} {
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr, path)
		content := string(data)
		assert.Contains(t, content, "## Autopus Branding", path)
		assert.Contains(t, content, "🐙 Autopus ─────────────────────────", path)
		if filepath.Base(path) == "SKILL.md" {
			assert.Contains(t, content, "## Codex Invocation", path)
			assert.Contains(t, content, "thin router", path)
		}
		for _, token := range banned {
			assert.NotContains(t, content, token, path)
		}
	}

	for _, name := range []string{
		"auto-setup",
		"auto-status",
		"auto-goal",
		"auto-update",
		"auto-plan",
		"auto-go",
		"auto-fix",
		"auto-review",
		"auto-sync",
		"auto-idea",
		"auto-map",
		"auto-why",
		"auto-verify",
		"auto-secure",
		"auto-test",
		"auto-qa",
		"auto-dev",
		"auto-canary",
		"auto-doctor",
	} {
		for _, path := range []string{
			filepath.Join(dir, ".agents", "skills", name, "SKILL.md"),
			filepath.Join(dir, ".codex", "prompts", name+".md"),
		} {
			data, readErr := os.ReadFile(path)
			require.NoError(t, readErr, path)
			content := string(data)
			assert.Contains(t, content, "## Autopus Branding", path)
			assert.Contains(t, content, "🐙 Autopus ─────────────────────────", path)
			for _, token := range banned {
				assert.NotContains(t, content, token, path)
			}
		}
		_, statErr := os.Stat(filepath.Join(dir, ".autopus", "plugins", "auto", "skills", name, "SKILL.md"))
		assert.True(t, os.IsNotExist(statErr), "plugin-local workflow shim should not exist for %s", name)
	}

	agentEntries, err := os.ReadDir(filepath.Join(dir, ".codex", "agents"))
	require.NoError(t, err)
	agentBanned := []string{"TeamCreate", "TeamDelete", "SendMessage", "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"}
	for _, entry := range agentEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		path := filepath.Join(dir, ".codex", "agents", entry.Name())
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr, path)
		for _, token := range agentBanned {
			assert.NotContains(t, string(data), token, path)
		}
	}

	autoIdeaSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-idea", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoIdeaSkill), "auto orchestra brainstorm")
	assert.Contains(t, string(autoIdeaSkill), "Clarification Ledger")
	assert.Contains(t, string(autoIdeaSkill), "Current understanding")
	assert.Contains(t, string(autoIdeaSkill), "Blocked decision")
	assert.Contains(t, string(autoIdeaSkill), "Recommended answer")
	assert.Contains(t, string(autoIdeaSkill), "Question")
	assert.Contains(t, string(autoIdeaSkill), "Question Audit")
	assert.Contains(t, string(autoIdeaSkill), "question_transport")
	assert.Contains(t, string(autoIdeaSkill), "scope_boundary")
	assert.Contains(t, string(autoIdeaSkill), "brownfield_impact")
	assert.Contains(t, string(autoIdeaSkill), "Outcome Lock")
	assert.Contains(t, string(autoIdeaSkill), "Evolution Ideas")
	assert.Contains(t, string(autoIdeaSkill), "Visual Brief")
	assert.Contains(t, string(autoIdeaSkill), "wireframe")
	assert.Contains(t, string(autoIdeaSkill), "product-discovery")
	assert.Contains(t, string(autoIdeaSkill), "Sequential Thinking으로 fallback할까요?")
	assert.Contains(t, string(autoIdeaSkill), "Pre-Completion Verification")

	autoSetupSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-setup", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoSetupSkill), "explorer")
	assert.Contains(t, string(autoSetupSkill), "ARCHITECTURE.md")
	assert.Contains(t, string(autoSetupSkill), "First Win Guidance")

	autoPlanSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-plan", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoPlanSkill), "auto spec review {SPEC-ID}")
	assert.Contains(t, string(autoPlanSkill), "review_gate.enabled")
	assert.Contains(t, string(autoPlanSkill), "Clarification Ledger")
	assert.Contains(t, string(autoPlanSkill), "Plan Intent Ledger")
	assert.Contains(t, string(autoPlanSkill), "Question Audit")
	assert.Contains(t, string(autoPlanSkill), "answered")
	assert.Contains(t, string(autoPlanSkill), "assumed")
	assert.Contains(t, string(autoPlanSkill), "deferred")
	assert.Contains(t, string(autoPlanSkill), "Outcome Lock")
	assert.Contains(t, string(autoPlanSkill), "Completion Debt")
	assert.Contains(t, string(autoPlanSkill), "Evolution Ideas")
	assert.Contains(t, string(autoPlanSkill), "Sibling SPEC Decision")
	assert.Contains(t, string(autoPlanSkill), "Visual Planning Brief")
	assert.Contains(t, string(autoPlanSkill), "sequence/data-flow")

	autoGoSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-go", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoGoSkill), "명시적 승인")
	assert.Contains(t, string(autoGoSkill), ".codex/skills/agent-pipeline.md")
	assert.Contains(t, string(autoGoSkill), "draft")
	assert.Contains(t, string(autoGoSkill), "autopus.yaml")
	assert.Contains(t, string(autoGoSkill), "spec.review_gate.enabled")
	assert.Contains(t, string(autoGoSkill), "max_revisions")
	assert.Contains(t, string(autoGoSkill), "approved")
	assert.Contains(t, string(autoGoSkill), "재귀 auto-chain")
	assert.Contains(t, string(autoGoSkill), "SPEC Path Resolution")
	assert.Contains(t, string(autoGoSkill), "WORKING_DIR")
	assert.Contains(t, string(autoGoSkill), "Completion Handoff Gates")
	assert.Contains(t, string(autoGoSkill), "`next_required_step`")
	assert.Contains(t, string(autoGoSkill), "`next_command`")
	assert.Contains(t, string(autoGoSkill), "`auto_progression_state`")
	assert.Contains(t, string(autoGoSkill), "`--loop`여도 handoff를 생략하지 않습니다")
	assert.Contains(t, string(autoGoSkill), "Autonomous Review Loop Contract")
	assert.Contains(t, string(autoGoSkill), "fix -> validate -> test -> review verify")
	assert.Contains(t, string(autoGoSkill), "terminal handoff는 `@auto sync {SPEC-ID}` 까지입니다")
	assert.Contains(t, string(autoGoSkill), "Sync Readiness Gate")
	assert.Contains(t, string(autoGoSkill), "completion_verdict_preview")
	assert.Contains(t, string(autoGoSkill), "spec_status_after_go")
	assert.Contains(t, string(autoGoSkill), "Codex native `multi_agent`")
	assert.Contains(t, string(autoGoSkill), "`/goal` active state")

	autoGoPrompt, err := os.ReadFile(filepath.Join(dir, ".codex", "prompts", "auto-go.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoGoPrompt), "Autonomous Review Loop Contract")
	assert.Contains(t, string(autoGoPrompt), "## Review Gate Resolution")
	assert.Contains(t, string(autoGoPrompt), "spec.review_gate.enabled")
	assert.Contains(t, string(autoGoPrompt), "review retry budget이 남아 있는 동안에는 사용자에게 수동 수정")
	assert.Contains(t, string(autoGoPrompt), "handoff는 terminal state에서만 사용합니다")
	assert.Contains(t, string(autoGoPrompt), "Sync Readiness Gate")
	assert.Contains(t, string(autoGoPrompt), "completion_verdict_preview")
	assert.Contains(t, string(autoGoPrompt), "team_mode: codex_multi_agent")
	assert.Contains(t, string(autoGoPrompt), "goal_status")

	autoGoCodexSkill, err := os.ReadFile(filepath.Join(dir, ".codex", "skills", "auto-go.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoGoCodexSkill), "Autonomous Review Loop Contract")
	assert.Contains(t, string(autoGoCodexSkill), "retry budget 소진 또는 circuit break 후 재개")
	assert.Contains(t, string(autoGoCodexSkill), "다음 단계: `@auto sync {SPEC-ID}`")

	autoReviewSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-review", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoReviewSkill), "repair loop 입력입니다")
	assert.Contains(t, string(autoReviewSkill), "standalone `@auto review`")

	autoReviewPrompt, err := os.ReadFile(filepath.Join(dir, ".codex", "prompts", "auto-review.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoReviewPrompt), "`@auto go --auto --loop` 내부 review라면")
	assert.Contains(t, string(autoReviewPrompt), "같은 invocation 안의 fixer/executor 단계로 되돌리세요")

	autoReviewCodexSkill, err := os.ReadFile(filepath.Join(dir, ".codex", "skills", "auto-review.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoReviewCodexSkill), "repair loop 입력입니다")
	assert.Contains(t, string(autoReviewCodexSkill), "recommend `@auto fix \"{specific issue}\"` or `@auto go {SPEC-ID} --continue`")

	autoSyncSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-sync", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoSyncSkill), "ARCHITECTURE.md")
	assert.Contains(t, string(autoSyncSkill), "@AX Lifecycle Management")
	assert.Contains(t, string(autoSyncSkill), "2-Phase Commit")
	assert.Contains(t, string(autoSyncSkill), "## Completion Gates")
	assert.Contains(t, string(autoSyncSkill), "@AX: no-op")
	assert.Contains(t, string(autoSyncSkill), "commit hash")
	assert.Contains(t, string(autoSyncSkill), "sync를 completed로 선언하지 않습니다")
	assert.Contains(t, string(autoSyncSkill), "## Completion Verdict")
	assert.Contains(t, string(autoSyncSkill), "Evolution Ideas: surfaced as optional, not scheduled")

	autoPrompt, err := os.ReadFile(filepath.Join(dir, ".codex", "prompts", "auto.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoPrompt), "하네스 기본값과 제약을 명시적으로 설명")
	assert.Contains(t, string(autoPrompt), "## Router Execution Contract")
	assert.Contains(t, string(autoPrompt), "## Context Load")
	assert.Contains(t, string(autoPrompt), "## SPEC Path Resolution")
	assert.Contains(t, string(autoPrompt), "ARCHITECTURE.md")
	assert.Contains(t, string(autoPrompt), "`setup`")
	assert.Contains(t, string(autoPrompt), "`goal`")
	assert.Contains(t, string(autoPrompt), "`doctor`")
	assert.Contains(t, string(autoPrompt), "`/goal`은 Codex thread-level 목표 기능입니다")
	assert.Contains(t, string(autoPrompt), "native `multi_agent` 도구 기반")
	assert.NotContains(t, string(autoPrompt), "`.agents/skills/auto/SKILL.md`의 최신 라우터 규칙을 우선")

	autoStatusSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-status", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoStatusSkill), "auto status")

	autoGoalSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-goal", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoGoalSkill), "Codex Goal Wrapper")
	assert.Contains(t, string(autoGoalSkill), "`@auto goal \"<objective>\" [--budget N]`")
	assert.Contains(t, string(autoGoalSkill), "`get_goal`")
	assert.Contains(t, string(autoGoalSkill), "`create_goal`")
	assert.Contains(t, string(autoGoalSkill), "`update_goal`")
	assert.Contains(t, string(autoGoalSkill), "not an ADK persisted state")

	autoGoalPrompt, err := os.ReadFile(filepath.Join(dir, ".codex", "prompts", "auto-goal.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoGoalPrompt), "`/goal` thread 기능")
	assert.Contains(t, string(autoGoalPrompt), "goal_status")
	assert.Contains(t, string(autoGoalPrompt), "`/goal clear`")

	autoDoctorSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-doctor", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoDoctorSkill), "auto doctor")

	autoMapSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-map", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoMapSkill), "spawn_agent(...)")

	agentTeamsSkill, err := os.ReadFile(filepath.Join(dir, ".codex", "skills", "agent-teams.md"))
	require.NoError(t, err)
	assert.Contains(t, string(agentTeamsSkill), "@auto go --auto")
	assert.Contains(t, string(agentTeamsSkill), "Codex Team Mode Skill")
	assert.Contains(t, string(agentTeamsSkill), "Lead/Builder/Guardian")
	assert.Contains(t, string(agentTeamsSkill), "`create_goal`")
	assert.NotContains(t, string(agentTeamsSkill), "TeamCreate")
	assert.NotContains(t, string(agentTeamsSkill), "SendMessage")

	agentPipelineSkill, err := os.ReadFile(filepath.Join(dir, ".codex", "skills", "agent-pipeline.md"))
	require.NoError(t, err)
	assert.Contains(t, string(agentPipelineSkill), "Context7 MCP")
	assert.Contains(t, string(agentPipelineSkill), "web search")
	assert.Contains(t, string(agentPipelineSkill), "do not ask the user to manually fix")
	assert.Contains(t, string(agentPipelineSkill), "Under `--auto --loop`, keep this repair -> validate -> verify cycle inside the same session")
	assert.Contains(t, string(agentPipelineSkill), "While review retries remain, unresolved findings are not a terminal handoff")
	assert.Contains(t, string(agentPipelineSkill), "Sync Readiness Gate")
	assert.Contains(t, string(agentPipelineSkill), "spec_status_after_go")
	assert.Contains(t, string(agentPipelineSkill), "Goal Integration")
	assert.Contains(t, string(agentPipelineSkill), "Codex team profile")
}

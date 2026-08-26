package omp

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ompWorkflowBodyContract struct {
	name             string
	source           string
	requiredSections []string
	uniqueSemantics  []string
	coordination     bool
}

func TestOMP002_WorkflowContract_DetailedBodiesMatchCanonicalOracles(t *testing.T) {
	t.Parallel()

	contracts := ompWorkflowBodyContracts()
	wantNames := map[string]bool{}
	for _, contract := range contracts {
		wantNames[contract.name] = true
	}
	require.Len(t, wantNames, 19, "the detailed workflow contract must be an exact nineteen-name set")

	gotNames := map[string]bool{}
	gotSpecs := map[string]workflowSpec{}
	for _, spec := range workflowSpecs {
		if spec.Name == "auto" {
			continue
		}
		gotNames[spec.Name] = true
		gotSpecs[spec.Name] = spec
	}
	assert.Equal(t, wantNames, gotNames, "workflowSpecs detailed names drifted from the independent oracle")

	a := NewWithRoot(t.TempDir())
	for _, contract := range contracts {
		contract := contract
		t.Run(contract.name, func(t *testing.T) {
			t.Parallel()

			spec, ok := gotSpecs[contract.name]
			require.True(t, ok)
			assert.Equal(t, contract.source, ompWorkflowCanonicalSource(spec))

			content, err := a.renderWorkflowSkill(spec, configForOMP())
			require.NoError(t, err)
			frontmatter, body := splitOMPFrontmatter(content)
			assert.Contains(t, frontmatter, "name: "+contract.name)
			assertOMPWorkflowInvocation(t, body, contract.name)

			headings := ompWorkflowHeadingSet(body)
			for _, section := range contract.requiredSections {
				assert.True(t, headings[section], "missing canonical section %q", section)
			}
			for _, semantic := range contract.uniqueSemantics {
				assert.Contains(t, body, semantic, "missing workflow-specific semantic contract")
			}
			if contract.name == "auto-setup" {
				for _, glob := range ompIntentionalPlatformRootGlobs() {
					assert.Equal(t, 1, strings.Count(body, glob),
						"auto-setup must retain the intentional exclusion inventory glob %s", glob)
				}
			}
			if contract.coordination {
				assertOMPNativeCoordinationContract(t, body)
			} else {
				assert.NotContains(t, body, "## OMP Coordination Contract",
					"non-delegating workflow must not duplicate the coordination contract")
			}
			sweepBody := stripOMPIntentionalPlatformRootGlobs(body)
			for _, token := range ompWorkflowForbiddenTokens() {
				assert.NotContains(t, sweepBody, token, "foreign or legacy token survived normalization")
			}
		})
	}
}

func ompWorkflowBodyContracts() []ompWorkflowBodyContract {
	coordinationWorkflows := map[string]bool{
		"auto-setup": true,
		"auto-plan":  true,
		"auto-go":    true,
		"auto-sync":  true,
		"auto-idea":  true,
	}
	compact := func(name string, semantics ...string) ompWorkflowBodyContract {
		sections := []string{"## OMP Invocation", "## 실행 계약"}
		if name == "auto-test" {
			sections = append(sections, "## Context Profile: test")
		}
		return ompWorkflowBodyContract{
			name: name, source: "compact:" + name, requiredSections: sections,
			uniqueSemantics: semantics,
		}
	}
	template := func(name, source string, sections, semantics []string) ompWorkflowBodyContract {
		return ompWorkflowBodyContract{
			name: name, source: source, requiredSections: append([]string{"## OMP Invocation"}, sections...),
			uniqueSemantics: semantics, coordination: coordinationWorkflows[name],
		}
	}
	return []ompWorkflowBodyContract{
		template("auto-setup", "codex/skills/auto-setup.md.tmpl",
			[]string{"## Workspace Folder Boundary", "## 실행 순서", "## Completion Message"}, []string{"ARCHITECTURE.md", ".autopus/project/"}),
		compact("auto-status", "SPEC lifecycle", "module 상태", "execution-owner.json", "native `hub`", "user-session roots"),
		compact("auto-goal", "goal surface", "persisted state"),
		compact("auto-update", "harness surface", "source-of-truth 경계"),
		template("auto-plan", "codex/skills/auto-plan.md.tmpl",
			[]string{"## Context Profile: plan", "## 실행 순서", "## 요구사항 형식 (EARS)"}, []string{"Clarification Ledger", "auto spec validate"}),
		template("auto-go", "codex/skills/auto-go.md.tmpl",
			[]string{"## Context Profile: go", "## 구현 절차", "## 실행 계약", "## 품질 기준", "## Execution Owner Control Plane"},
			[]string{"RED: 실패 테스트", ".omp/skills/agent-pipeline/SKILL.md", `"outputSchema"`, "single DAG owner invariant",
				"--execution-owner omp|orca", "pipeline_execution_owner_receipt.v1", "supervised Orca workers"}),
		template("auto-fix", "codex/skills/auto-fix.md.tmpl",
			[]string{"## 절차", "## 규칙"}, []string{"버그 재현 테스트", "최소 코드 변경"}),
		template("auto-review", "codex/skills/auto-review.md.tmpl",
			[]string{"## Canonical Semantic Contract", "## TRUST 5 기준", "## Output Sections"}, []string{"REQUEST_CHANGES", "Security"}),
		template("auto-sync", "codex/skills/auto-sync.md.tmpl",
			[]string{"## 동기화 항목", "## Completion Gates", "## Completion Verdict"}, []string{"SPEC 상태 → completed", "@AX Lifecycle Management"}),
		template("auto-idea", "codex/skills/auto-idea.md.tmpl",
			[]string{"## Canonical Semantic Contract", "## 5단계 파이프라인", "## Clarification Ledger"}, []string{"ICE 스코어링", "Intent Brief"}),
		compact("auto-map", "entrypoint", "의존성"),
		compact("auto-why", "Lore, SPEC, ARCHITECTURE", "근거 파일"),
		compact("auto-verify", "Playwright", "visual state"),
		compact("auto-secure", "OWASP", "finding"),
		compact("auto-test", "scenarios.md", "시나리오별 PASS / WARN / FAIL"),
		template("auto-qa", "codex/skills/auto-qa.md.tmpl",
			[]string{"## Runner Boundary", "## Command Selection", "## Guardrails"}, []string{"QAMESH", "Journey Pack"}),
		compact("auto-dev", "`auto plan` → `auto go` → `auto sync`", "첫 실패 단계"),
		template("auto-canary", "codex/skills/auto-canary.md.tmpl",
			[]string{"## Context Profile: canary", "## Scope Boundary", "## 실행 순서", "## 판정 기준"}, []string{"post-deploy", ".autopus/canary/latest.json"}),
		compact("auto-doctor", "platform wiring", "expected/got"),
	}
}

func ompWorkflowCanonicalSource(spec workflowSpec) string {
	if spec.SkillPath != "" {
		return spec.SkillPath
	}
	return "compact:" + spec.Name
}

func assertOMPWorkflowInvocation(t *testing.T, body, name string) {
	t.Helper()
	subcommand := strings.TrimPrefix(name, "auto-")
	for _, line := range []string{
		"- `/auto " + subcommand + " ...`",
		"- `/" + name + " ...`",
		"- Load detail skill `" + name + "` for either entrypoint.",
	} {
		assert.Equal(t, 1, strings.Count(body, line), "invocation contract line must occur exactly once: %s", line)
	}
}

func ompWorkflowHeadingSet(body string) map[string]bool {
	headings := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			headings[line] = true
		}
	}
	return headings
}

func assertOMPNativeCoordinationContract(t *testing.T, body string) {
	t.Helper()
	assert.Equal(t, 1, strings.Count(body, "## OMP Coordination Contract"))
	for _, field := range []string{
		`"i"`, `"context"`, `"tasks"`, `"name"`, `"task"`,
		`"outputSchema"`, `"schemaMode"`,
		`"owned_paths"`, `"changed_files"`, `"verification"`,
		`"blockers"`, `"next_required_step"`,
	} {
		assert.Contains(t, body, field, "missing native OMP field %s", field)
	}
	assert.Contains(t, body, "`agent`", "custom agent field rule must be explicit")
	assert.Contains(t, body, "non-isolated or otherwise revivable")
	assert.Contains(t, body, "isolated worker is terminal")
	assert.Contains(t, body, "new explicitly named")
	assert.Contains(t, body,
		`"required": ["owned_paths", "changed_files", "verification", "blockers", "next_required_step"]`)
	assert.Contains(t, body, `"schemaMode": "strict"`)
	assert.Contains(t, body, `{"i":"Following up with an existing worker","op":"send","to":"<same agent id>","message":"<follow-up>"}`)
	assert.Contains(t, body, `{"i":"Updating parent-owned progress","op":"init","list":[{"phase":"Implementation","items":["..."]}]}`)
	assert.Contains(t, body, `{"i":"Updating parent-owned progress","op":"start","task":"<exact task content>"}`)
	assert.Contains(t, body, "that same id")
	assert.Contains(t, body, "owner `omp`")
	assert.Contains(t, body, "owner `orca`")
	assert.Contains(t, body, "--execution-owner orca")
	assert.Contains(t, body, "orca skills get orchestration --full")
	assert.Contains(t, body, "single DAG owner invariant")
	assert.NotContains(t, body, "OMP-local")
	assert.NotContains(t, body, "Orca-supervised")
}

func ompIntentionalPlatformRootGlobs() []string {
	return []string{".codex/**", ".claude/**", ".gemini/**", ".opencode/**"}
}

var ompIntentionalPlatformRootGlobRe = regexp.MustCompile(
	`\.(codex|claude|gemini|opencode)/\*\*([^/A-Za-z0-9_-]|$)`,
)

func stripOMPIntentionalPlatformRootGlobs(body string) string {
	return ompIntentionalPlatformRootGlobRe.ReplaceAllString(body, "${2}")
}

func ompWorkflowForbiddenTokens() []string {
	return []string{
		".codex/", ".claude/", ".opencode/", ".gemini/",
		"Claude Code", "Claude", "Codex", "OpenCode", "Gemini",
		"@auto ", "@auto-",
		"Agent(", "subagent_type", "prompt =", "prompt=", "task tool", "task(...)",
		"spawn_agent", "multi_agent", "send_input", "wait_agent", "close_agent",
		"TodoWrite", "TaskCreate", "TaskUpdate", "TaskList", "TaskGet",
		"TeamCreate", "TeamDelete", "SendMessage", "ToolSearch",
		"AskUserQuestion", "request_user_input",
		`isolation: "worktree"`, `isolation = "worktree"`, "auto pipeline worktree",
	}
}

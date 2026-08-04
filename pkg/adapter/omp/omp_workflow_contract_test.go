package omp

import (
	"crypto/sha256"
	"fmt"
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
	bodySHA256       string
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
			for _, token := range ompWorkflowForbiddenTokens() {
				assert.NotContains(t, body, token, "foreign-platform token survived normalization")
			}

			gotHash := fmt.Sprintf("%x", sha256.Sum256([]byte(normalizeOMPWorkflowOracleBody(body))))
			assert.Equal(t, contract.bodySHA256, gotHash, "normalized detailed body drifted")
		})
	}
}

func ompWorkflowBodyContracts() []ompWorkflowBodyContract {
	compact := func(name, hash string, semantics ...string) ompWorkflowBodyContract {
		sections := []string{"## OMP Invocation", "## 실행 계약"}
		if name == "auto-test" {
			sections = append(sections, "## Context Profile: test")
		}
		return ompWorkflowBodyContract{
			name: name, source: "compact:" + name, requiredSections: sections,
			uniqueSemantics: semantics, bodySHA256: hash,
		}
	}
	template := func(name, source, hash string, sections, semantics []string) ompWorkflowBodyContract {
		return ompWorkflowBodyContract{
			name: name, source: source, requiredSections: append([]string{"## OMP Invocation"}, sections...),
			uniqueSemantics: semantics, bodySHA256: hash,
		}
	}
	return []ompWorkflowBodyContract{
		template("auto-setup", "codex/skills/auto-setup.md.tmpl", "328dcfcfbfbb0069cd76ddafdff20d56be972cb0a92b3257907079b1875d2e39",
			[]string{"## Workspace Folder Boundary", "## 실행 순서", "## Completion Message"}, []string{"ARCHITECTURE.md", ".autopus/project/"}),
		compact("auto-status", "ac67f2fa33961367bec251f1405e9e83bac9739f23e980dbd723b6de96a583d1", "SPEC lifecycle", "module 상태"),
		compact("auto-goal", "3b1a8a93c3bef95f25f004e6c4b255d4afdfbd680d7c333802dfee3a459e262a", "goal surface", "persisted state"),
		compact("auto-update", "0b3abe8503748ddeeffcfd4adf1c6a94b4703b2fad3927ac2a5f405e6825ac03", "harness surface", "source-of-truth 경계"),
		template("auto-plan", "codex/skills/auto-plan.md.tmpl", "964c57a4b1d91da7290f3167bbc96b3810043db8969ad82ff9752706e3f3fc75",
			[]string{"## Context Profile: plan", "## 실행 순서", "## 요구사항 형식 (EARS)"}, []string{"Clarification Ledger", "auto spec validate"}),
		template("auto-go", "codex/skills/auto-go.md.tmpl", "42d5e9582e8e5c18a0ad283d9a3914787cea29c1867c1ba114a4f6345e78be6c",
			[]string{"## Context Profile: go", "## 구현 절차", "## 실행 계약", "## 품질 기준"}, []string{"RED: 실패 테스트", ".agents/skills/agent-pipeline/SKILL.md"}),
		template("auto-fix", "codex/skills/auto-fix.md.tmpl", "d9c956c3e4220909b81a314d5ae8ceb7958d0beed7c406f76b0707f27b3a72ae",
			[]string{"## 절차", "## 규칙"}, []string{"버그 재현 테스트", "최소 코드 변경"}),
		template("auto-review", "codex/skills/auto-review.md.tmpl", "2eace041ee76796931aebb07abf8d9336120006e34b2b55522f576bd0e3a11ba",
			[]string{"## Canonical Semantic Contract", "## TRUST 5 기준", "## Output Sections"}, []string{"REQUEST_CHANGES", "Security"}),
		template("auto-sync", "codex/skills/auto-sync.md.tmpl", "39c50ef8a60e2ad3efc411afbb8b533fd41508c93b7c61bc4f4810ac2a384f63",
			[]string{"## 동기화 항목", "## Completion Gates", "## Completion Verdict"}, []string{"SPEC 상태 → completed", "@AX Lifecycle Management"}),
		template("auto-idea", "codex/skills/auto-idea.md.tmpl", "9348060db6e63a5053668a7157070fe095cbba601c602891b3d478ce4d1e4a69",
			[]string{"## Canonical Semantic Contract", "## 5단계 파이프라인", "## Clarification Ledger"}, []string{"ICE 스코어링", "Intent Brief"}),
		compact("auto-map", "4a49ba2eb5a4eb57ea1e3cbf004a58c931aac60e8157994171a069093c013e25", "entrypoint", "의존성"),
		compact("auto-why", "f24ad614106709de2d9a2f6bf6546a4443fb729b46ce543e5892168e77d5bf29", "Lore, SPEC, ARCHITECTURE", "근거 파일"),
		compact("auto-verify", "43624b462376abaf79f6559c27b019f4c870fe562f9884b29c3657fd1400b71e", "Playwright", "visual state"),
		compact("auto-secure", "f09804b06effdc12864273e37f7304b47fdb4efa4ca5e9aad14ff23fe9ecb938", "OWASP", "finding"),
		compact("auto-test", "8dee1081fe3e481e2bc18b02c1e813fe7dc0293bb54748ad9c1fd387cc55217d", "scenarios.md", "시나리오별 PASS / WARN / FAIL"),
		template("auto-qa", "codex/skills/auto-qa.md.tmpl", "cadccc539fb37a1b8040f3f6d5435c74537a9f9317d2ac8a5947844cce9e1998",
			[]string{"## Runner Boundary", "## Command Selection", "## Guardrails"}, []string{"QAMESH", "Journey Pack"}),
		compact("auto-dev", "589b45a6d1e7424305abd81788877d02ee2e75d73d479c171782afddd9d9e7a0", "`auto plan` → `auto go` → `auto sync`", "첫 실패 단계"),
		template("auto-canary", "codex/skills/auto-canary.md.tmpl", "5d79e88f704c229806e9f67bfa487fd16c7bb66da27976a9dc7c391d205b1f2e",
			[]string{"## Context Profile: canary", "## Scope Boundary", "## 실행 순서", "## 판정 기준"}, []string{"post-deploy", ".autopus/canary/latest.json"}),
		compact("auto-doctor", "e0fa6c8570df8344a7b4455696149870b137c4c570abe32772953e04398c180b", "platform wiring", "expected/got"),
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

func ompWorkflowForbiddenTokens() []string {
	return []string{
		".codex/", ".claude/", ".opencode/", ".gemini/",
		"@auto ", "@auto-", "spawn_agent", "request_user_input",
	}
}

func normalizeOMPWorkflowOracleBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

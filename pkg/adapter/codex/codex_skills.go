package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

// renderSkillTemplates reads Codex skill templates from embedded FS,
// renders them, and writes to .codex/skills/.
func (a *Adapter) renderSkillTemplates(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir("codex/skills")
	if err != nil {
		return nil, fmt.Errorf("코덱스 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		name := entry.Name()
		skillFile := strings.TrimSuffix(name, ".tmpl")

		tmplContent, err := templates.FS.ReadFile("codex/skills/" + name)
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 읽기 실패 %s: %w", name, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			if strings.HasPrefix(skillFile, "auto-") {
				return nil, fmt.Errorf("코덱스 스킬 템플릿 렌더링 실패 %s: %w", name, err)
			}
			// Non-auto skill docs can legitimately contain raw `{{ ... }}` examples
			// such as GitHub Actions expressions. Preserve the template verbatim.
			rendered = string(tmplContent)
		}
		rendered = normalizeCodexInvocationBody(rendered)
		rendered = normalizeCodexHelperPaths(rendered)
		rendered = normalizeCodexToolingBody(rendered)

		targetPath := filepath.Join(a.root, ".codex", "skills", skillFile)
		if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
			return nil, fmt.Errorf("코덱스 스킬 파일 쓰기 실패 %s: %w", targetPath, err)
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "skills", skillFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	// Extended skills from content/skills/ via transformer
	extFiles, err := a.renderExtendedSkills()
	if err != nil {
		return nil, fmt.Errorf("extended skill rendering failed: %w", err)
	}
	for _, ef := range extFiles {
		targetPath := filepath.Join(a.root, ef.TargetPath)
		if err := os.WriteFile(targetPath, ef.Content, 0644); err != nil {
			return nil, fmt.Errorf("extended skill write failed %s: %w", targetPath, err)
		}
	}
	files = append(files, extFiles...)

	return files, nil
}

// agentsMDTemplate is the AGENTS.md AUTOPUS section template.
const agentsMDTemplate = `# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: {{.ProjectName}}
- **모드**: {{.Mode}}
- **플랫폼**: {{join ", " .Platforms}}

## Installed Components

{{if contains (join ", " .Platforms) "codex"}}- Codex Rules: .codex/rules/autopus/
- Codex Skills: .codex/skills/
- Codex Agents: .codex/agents/
- Codex Config: config.toml
{{end}}{{if contains (join ", " .Platforms) "opencode"}}- OpenCode Rules: .opencode/rules/autopus/
- OpenCode Commands: .opencode/commands/
- OpenCode Agents: .opencode/agents/
- OpenCode Plugins: .opencode/plugins/
{{end}}{{if contains (join ", " .Platforms) "codex"}}- Shared Agent Skills: .agents/skills/
- Plugin Marketplace: .agents/plugins/marketplace.json
{{else if contains (join ", " .Platforms) "opencode"}}- Shared Skills: .agents/skills/
{{end}}

## Language Policy

IMPORTANT: Follow these language settings strictly for all work in this project.

- **Code comments**: {{.Language.Comments}}
- **Commit messages**: {{.Language.Commits}}
- **AI responses**: {{.Language.AIResponses}}

## Execution Model

{{if contains (join ", " .Platforms) "codex"}}- **Codex**: 하네스 기본값은 spawn_agent(...) 기반 subagent-first 입니다.
- **Codex --auto**: @auto ... --auto 가 포함되면, 기본 subagent pipeline 진행에 대한 명시적 승인으로 해석합니다.
- **Codex Runtime Caveat**: 현재 세션의 Codex 런타임 정책이 암묵적 spawn_agent(...) 호출을 제한하면, 조용히 단일 세션으로 폴백하지 말고 그 제약을 명시적으로 알린 뒤 사용자의 서브에이전트 opt-in 또는 --solo 선택을 받으세요.
- **Codex --team**: 미래의 native multi-agent surface를 위한 reserved compatibility flag입니다.
{{end}}{{if contains (join ", " .Platforms) "opencode"}}- **OpenCode**: 기본 실행 모델은 task(...) 기반 subagent-first 입니다.
- **OpenCode Invocation**: /auto <subcommand> ... 또는 /auto-<subcommand> ... alias를 사용합니다.
{{end}}

## Core Guidelines

{{if contains (join ", " .Platforms) "codex"}}### Supervisor Contract

IMPORTANT: 메인 세션은 얇은 라우터가 아니라 phase/gate를 관리하는 supervisor입니다. 각 단계마다 필수 단계, skip 조건, retry 한도, 다음 필수 단계를 명확히 유지하세요.

### Subagent Delegation

IMPORTANT: 3개 이상 파일 수정, 다중 도메인 변경, 또는 신규 코드 200줄 초과가 예상되면 기본적으로 서브에이전트를 사용하세요. 단, 읽기 위주 탐색/리서치/테스트 분석은 병렬 fan-out을 우선하고, 쓰기 위주 구현은 파일 소유권이 겹치면 순차 실행으로 전환하세요.

### Worker Contracts

IMPORTANT: 각 worker 프롬프트에는 반드시 소유 파일/모듈, 수정 금지 범위, 완료 기준, 반환 형식을 포함하세요. 최소 반환 필드는 ` + "`owned_paths`, `changed_files`, `verification`, `blockers`, `next_required_step`" + ` 입니다.

### Review Convergence

IMPORTANT: 리뷰는 discovery와 verification을 분리하세요. 첫 리뷰는 finding discovery에 집중하고, 재시도는 열린 finding 해결 여부만 diff 기준으로 확인하세요. 같은 범위를 무한 재탐색하지 마세요.

### File Size Limit

IMPORTANT: 생성 파일을 제외한 소스 파일은 300줄 이하를 유지하세요. 가능하면 200줄 이하를 목표로 분리하세요.

### Prompting Notes

IMPORTANT: 사용자가 계획만 요구한 경우를 제외하면, 긴 선행 계획만 출력하고 멈추지 마세요. 먼저 코드베이스를 확인하고, 필요한 경우 서브에이전트를 스폰한 뒤, 검증까지 이어서 진행하세요.

## Rules

See .codex/rules/autopus/ for Codex rule definitions.
See .codex/skills/agent-pipeline.md for phase and gate contracts.
See .codex/agents/ for Codex agent definitions.
{{end}}{{if contains (join ", " .Platforms) "opencode"}}See .opencode/rules/autopus/ for OpenCode rule definitions.
{{end}}
`

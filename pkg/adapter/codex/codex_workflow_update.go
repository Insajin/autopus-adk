package codex

func updateWorkflowBody(name, summary string) customWorkflowBody {
	prompt := compose(
		"# "+name+" — Harness Update",
		"",
		"## 설명",
		"",
		summary,
		"",
		"## 실행 원칙",
		"",
		"- 이 워크플로우는 `auto update` CLI wrapper입니다.",
		"- 현재 디렉터리가 메타 workspace이면 `auto update`가 workspace mode를 자동 감지합니다.",
		"- 사용자가 특정 repo 이름을 제공하면 `auto update <repo>` 형태로 전달합니다.",
		"- 사용자가 현재 repo만 원하면 `auto update --local`을 사용합니다.",
		"- 코딩 도구 내부 실행에서는 확인 프롬프트가 막히지 않도록 `auto --auto update`를 기본 실행 명령으로 사용합니다.",
		"",
		"## 실행 명령",
		"",
		"`auto --auto update`",
	)

	skill := compose(
		"# "+name+" — Harness Update",
		"",
		"## 설명",
		"",
		summary,
		"",
		"## Codex Invocation",
		"",
		"- `@auto update ...`",
		"- `$auto-update ...`",
		"- `$auto update ...`",
		"",
		"## 실행 순서",
		"",
		"1. project context와 현재 작업 디렉터리를 확인합니다.",
		"2. 먼저 `auto --auto update --plan`으로 smart target selection을 확인합니다.",
		"3. plan에 `missing autopus.yaml`이 있는 repo가 있고 workspace policy상 harness-managed가 명확하면 최소 `autopus.yaml`을 직접 생성합니다.",
		"4. 새 config는 root config의 mode/platforms/language를 보수적으로 상속하고 `project_name`은 repo 이름으로 둡니다.",
		"5. 적용이 필요한 경우 `auto --auto update` 또는 `auto --auto update <repo>`를 실행합니다.",
		"6. 결과에서 업데이트된 repo, 스킵된 repo, 추가 조치가 필요한 repo를 요약합니다.",
	)

	return customWorkflowBody{prompt: prompt, skill: skill}
}

package opencode

type workflowSpec struct {
	Name        string
	Description string
	PromptPath  string
}

var workflowSpecs = []workflowSpec{
	{
		Name:        "auto",
		Description: "Autopus 명령 라우터 — plan/go/fix/review/sync/canary/idea 서브커맨드를 해석합니다",
		PromptPath:  "codex/prompts/auto.md.tmpl",
	},
	{
		Name:        "auto-plan",
		Description: "SPEC 작성 — 코드베이스 분석 후 EARS 요구사항, 구현 계획, 인수 기준을 생성합니다",
		PromptPath:  "codex/prompts/auto-plan.md.tmpl",
	},
	{
		Name:        "auto-go",
		Description: "SPEC 구현 — SPEC 문서를 기반으로 코드를 구현합니다",
		PromptPath:  "codex/prompts/auto-go.md.tmpl",
	},
	{
		Name:        "auto-fix",
		Description: "버그 수정 — 재현과 최소 수정 중심으로 문제를 해결합니다",
		PromptPath:  "codex/prompts/auto-fix.md.tmpl",
	},
	{
		Name:        "auto-review",
		Description: "코드 리뷰 — TRUST 5 기준으로 변경된 코드를 리뷰합니다",
		PromptPath:  "codex/prompts/auto-review.md.tmpl",
	},
	{
		Name:        "auto-sync",
		Description: "문서 동기화 — 구현 이후 SPEC, CHANGELOG, 문서를 반영합니다",
		PromptPath:  "codex/prompts/auto-sync.md.tmpl",
	},
	{
		Name:        "auto-idea",
		Description: "아이디어 브레인스토밍 — 멀티 프로바이더 토론과 ICE 평가로 아이디어를 정리합니다",
		PromptPath:  "codex/prompts/auto-idea.md.tmpl",
	},
	{
		Name:        "auto-canary",
		Description: "배포 검증 — build, E2E, 브라우저 건강 검진을 실행합니다",
		PromptPath:  "codex/prompts/auto-canary.md.tmpl",
	},
}

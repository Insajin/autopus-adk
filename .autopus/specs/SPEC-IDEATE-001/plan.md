# SPEC-IDEATE-001 구현 계획

## 태스크 목록

- [x] T1: autopus.yaml 스키마에 brainstorm 커맨드 엔트리 추가
- [x] T2: pkg/config/defaults.go의 DefaultFullConfig에 brainstorm CommandEntry 추가
- [x] T3: internal/cli/orchestra_brainstorm.go 신규 파일 — newOrchestraBrainstormCmd() + buildBrainstormPrompt()
- [x] T4: .claude/commands/auto.md plan 파이프라인에 Step 1.5.1.5 브레인스토밍 단계 삽입
- [x] T5: .claude/skills/autopus/brainstorming.md에 멀티 프로바이더 워크플로우 섹션 추가
- [x] T6: 브레인스토밍 프롬프트 템플릿 작성 (SCAMPER + HMW + divergence-preserving judge + ICE 스코어링)
- [x] T7: T2-T3 단위 테스트 작성 (85%+ 커버리지)

## 에이전트 할당 테이블

| Task ID | Description | Agent | Mode | File Ownership |
|---------|-------------|-------|------|----------------|
| T1 | autopus.yaml brainstorm 엔트리 추가 | executor | sequential | autopus.yaml |
| T2 | defaults.go brainstorm CommandEntry 추가 | executor | parallel | pkg/config/defaults.go |
| T3 | orchestra_brainstorm.go 신규 파일 생성 | executor | parallel | internal/cli/orchestra_brainstorm.go |
| T4 | auto.md Step 1.5.1.5 삽입 | executor | sequential | .claude/commands/auto.md |
| T5 | brainstorming.md 멀티 프로바이더 섹션 추가 | executor | sequential | .claude/skills/autopus/brainstorming.md |
| T6 | 프롬프트 템플릿 (T3에 내장) | executor | — | (T3에 포함) |
| T7 | 테스트 작성 | tester | sequential | internal/cli/orchestra_brainstorm_test.go, pkg/config/defaults_test.go |

## 구현 전략

### 리뷰 반영 사항

1. **SPEC-ID 시점 해결**: Step 번호를 Step 1.5.1.5로 변경 (Step 1.5.1 SPEC-ID 생성 직후)
2. **debate 전략 적합성**: divergence-preserving judge prompt 사용 — 아이디어 필터링이 아닌 보강/통합 목적
3. **파일 분리**: orchestra.go (274줄)에 추가하지 않고 orchestra_brainstorm.go 별도 파일로 생성
4. **타임아웃 명확화**: per-provider per-round 기준
5. **ICE 스코어링 주체**: debate judge가 최종 통합 및 스코어링 수행
6. **컨텍스트 전파**: brainstorm.md 경로를 spec-writer 에이전트 프롬프트에 명시적 포함

### T1: autopus.yaml 수정

- `orchestra.commands` 맵에 `brainstorm` 키 추가
- Strategy: `debate`, Providers: `[claude, codex, gemini]`

### T2: defaults.go 수정

- `DefaultFullConfig().Orchestra.Commands` 맵에 brainstorm 엔트리 추가
- `CommandEntry{Strategy: "debate", Providers: []string{"claude", "codex", "gemini"}}`

### T3: orchestra_brainstorm.go 신규 파일

- `newOrchestraBrainstormCmd()` — Cobra 커맨드 정의
- `buildBrainstormPrompt(feature string) string` — SCAMPER 7관점 + HMW 질문 구조화 프롬프트
- 기존 `newOrchestraPlanCmd()` 패턴 참조
- `newOrchestraCmd()`에 `cmd.AddCommand(newOrchestraBrainstormCmd())` 등록

### T4: auto.md 파이프라인 수정

- Step 1.5.1 직후에 Step 1.5.1.5 삽입
- 조건: `MULTI_FLAG == true`일 때만 실행
- `auto orchestra brainstorm "{FEATURE_DESC}"` CLI 호출
- 결과를 `.autopus/specs/SPEC-{ID}/brainstorm.md`에 저장
- Step 1.5.2 PRD 생성 시 brainstorm.md 경로를 컨텍스트로 전달

### T5: brainstorming.md 스킬 확장

- `## Multi-Provider Brainstorming` 섹션 추가
- divergence-preserving judge prompt 정의
- brainstorm.md 출력 포맷 정의 (프로바이더별 아이디어 + 병합 결과 + ICE 스코어)

### T7: 테스트

- `internal/cli/orchestra_brainstorm_test.go`: brainstorm 서브커맨드 등록 확인, 프롬프트 빌더 테스트
- `pkg/config/defaults_test.go`: brainstorm 커맨드 엔트리 존재 확인

## 변경 범위

| 파일 | 변경 유형 | 예상 줄 수 |
|------|----------|-----------|
| `autopus.yaml` | 수정 | +3 |
| `pkg/config/defaults.go` | 수정 | +1 |
| `internal/cli/orchestra_brainstorm.go` | 신규 | ~60 |
| `internal/cli/orchestra.go` | 수정 | +1 (AddCommand) |
| `.claude/commands/auto.md` | 수정 | +50 |
| `.claude/skills/autopus/brainstorming.md` | 수정 | +40 |
| `internal/cli/orchestra_brainstorm_test.go` | 신규 | ~40 |
| `pkg/config/defaults_test.go` | 수정 | +10 |

총 예상 변경: ~205 줄

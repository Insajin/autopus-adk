# SPEC-LEARN-001 리서치

## 기존 코드 분석

### Lore 패키지 (참조 패턴)

- `pkg/lore/types.go` — LoreEntry 구조체 정의 패턴. 9개 트레일러 필드 + 메타데이터 구조. LearningEntry도 동일한 flat struct 패턴 사용.
- `pkg/lore/writer.go` — 파일 쓰기 패턴. JSONL append에 그대로 적용 가능.
- `pkg/lore/query.go` — 쿼리/필터링 패턴. 관련성 매칭에 참고.
- `pkg/lore/validator.go` — 입력 검증 패턴.

### Pipeline 패키지

- `pkg/pipeline/types.go` — CheckpointStatus enum (pending/in_progress/done/failed). Learning의 phase 필드와 연결 가능.
- `pkg/pipeline/checkpoint.go` — YAML 직렬화 패턴. Learning은 JSONL이므로 다르지만 파일 I/O 패턴 참조.

### Gate 시스템

- `internal/cli/gate.go` — `GateCheck()` 함수가 `GateResult`를 반환. Learning 기록 훅은 GateCheck 호출자 측에서 `GateResult.Passed == false`일 때 트리거해야 함. GateCheck 내부를 수정하지 않고, 호출 후 결과 기반으로 기록.
- `internal/cli/gate.go:knownGates` — 현재 `phase2`만 등록. Gate 2, Gate 3는 스킬 레벨에서 정의됨.

### CLI 구조

- `internal/cli/root.go` — cobra root 커맨드. `auto learn`을 여기에 서브커맨드로 등록.
- `internal/cli/lore.go` — `auto lore` 서브커맨드 구현 패턴. learn 커맨드도 동일 패턴.
- `internal/cli/pipeline.go` — checkpoint 경로 패턴 (`pipelineStateDir = ".autopus/pipeline-state"`). Learning은 `.autopus/learnings/`에 저장.

### 스킬 프롬프트

- `content/skills/agent-pipeline.md` — Phase 1 (Planning) 설명 위치(line 60~). "Phase 1.8: Doc Fetch" 이전에 "Phase 0.5: Learning Injection" 또는 Phase 1 planner 프롬프트에 직접 주입.
- `content/skills/debugging.md` — "2단계: 근본 원인 분석" 섹션 이전에 learnings 참조 단계 추가.
- `templates/claude/commands/auto-router.md.tmpl` — fix 서브커맨드 라우팅. learnings 참조 지시를 fix 핸들러에 추가.

## 설계 결정

### D1: JSONL vs SQLite vs YAML

**결정**: JSONL

| 옵션 | 장점 | 단점 |
|------|------|------|
| JSONL | append-only로 동시성 안전, git-friendly, 도구 호환 | 쿼리 시 전체 스캔 |
| SQLite | 빠른 쿼리, 인덱스 | git에서 바이너리, 의존성 추가, 병합 충돌 |
| YAML | 기존 checkpoint와 일관 | append 비효율, 대규모 시 파싱 느림 |

JSONL 선택 이유: (1) gstack과 동일한 포맷으로 검증됨 (2) append-only로 파이프라인 중 동시 쓰기에 안전 (3) git diff에서 변경 추적 용이. 항목 수가 수백 단위이므로 전체 스캔 비용은 무시 가능.

### D2: 기록 위치 — Go 바이너리 vs 스킬 레벨

**결정**: Go 바이너리 (pkg/learn/)

사용자 피드백 "기능은 Go 바이너리에 넣어 배포할 것"에 따라 store/query/prune 로직은 Go 바이너리에 구현. 스킬 레벨은 "프롬프트에 주입하라"는 지시만 추가.

### D3: 주입 방식 — CLI 출력 파이핑 vs 프롬프트 삽입

**결정**: 프롬프트 삽입 (스킬 레벨)

`auto learn query --spec SPEC-FOO-001 --limit 5 --format prompt` CLI 커맨드로 관련 learnings를 프롬프트 형태로 출력. 스킬에서 이 커맨드 결과를 planner 프롬프트에 삽입하도록 지시.

### D4: 관련성 매칭 — 벡터 검색 vs 키워드 매칭

**결정**: 키워드/경로 매칭 (단순)

벡터 검색은 임베딩 모델 의존성을 추가하고, 항목 수가 적어(수백) 키워드 매칭으로 충분. 가중치 스코어링: 파일 경로 완전 일치(10점), 패키지 접두어 일치(5점), 도메인 키워드 일치(2점), 최근 30일 보너스(+3점).

### D5: reuse_count 업데이트 — in-place vs append-new

**결정**: in-place 업데이트 (JSONL 재작성)

JSONL의 한 줄을 in-place 수정하는 것은 불가능하므로, 주입 시 전체 파일을 읽고 해당 항목의 reuse_count를 증가시킨 뒤 전체를 재작성. 항목 수가 적으므로 성능 이슈 없음. 대안으로 별도 카운트 파일을 두는 방법이 있으나 복잡도 증가 대비 이점 없음.

### D6: 파일 분리 — pipeline.jsonl vs patterns.jsonl

**결정**: 2파일 분리

- `pipeline.jsonl`: 파이프라인 실행 중 자동 생성되는 항목 (gate_fail, coverage_gap, review_issue, executor_error)
- `patterns.jsonl`: 수동 추가 항목 (manual) 및 향후 코드 패턴 자동 추출 항목

분리 이유: 자동 생성 항목은 prune 대상이지만, 수동 항목은 장기 보존되는 경우가 많음. prune 정책을 파일별로 다르게 적용 가능.

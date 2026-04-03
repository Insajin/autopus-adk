# SPEC-RETRO-001: Automated Retrospective (retro 서브커맨드)

**Status**: draft
**Created**: 2026-04-03
**Domain**: RETRO
**Target Version**: v0.16

## 목적

개발 사이클의 마지막 피드백 루프인 회고를 자동화한다. git history, SPEC 완료 이력, Lore 의사결정 데이터를 분석하여 "무엇이 잘 됐는가 / 무엇이 안 됐는가 / 다음에 무엇을 할 것인가"를 자동 생성한다.

canary가 "배포 직후 검증"이라면, retro는 "기간별 학습 축적"이다. SPEC-LEARN-001(Autopus 플랫폼 Learning System)과 연동하면 팀 전체의 지속적 개선 데이터를 자동 수집할 수 있다.

## 요구사항

### R1: 기간 설정
WHEN the user runs `/auto retro`, THE SYSTEM SHALL default to the last 7 days. WHEN `--since` and/or `--until` flags are provided, THE SYSTEM SHALL use the specified date range.

### R2: git log 분석
WHEN generating a retro, THE SYSTEM SHALL analyze git log for the given period and extract: total commit count, files changed, lines added/removed, and contributor statistics.

### R3: SPEC 완료 현황
WHEN generating a retro, THE SYSTEM SHALL scan all SPEC directories for SPECs with status `completed` and `Created` date within the retro period, and list them with titles.

### R4: Lore 패턴 분석
WHEN Lore entries exist in the git history for the retro period, THE SYSTEM SHALL extract decision patterns including: confidence distribution (low/medium/high), scope-risk distribution, most common constraints, and rejected alternatives.

### R5: 반복 실패 패턴 식별
WHEN Lore entries contain `Rejected` or `Not-Tested` trailers, THE SYSTEM SHALL identify recurring patterns and flag them as areas needing attention.

### R6: 회고 문서 생성
WHEN analysis completes, THE SYSTEM SHALL generate `.autopus/retro/RETRO-{YYYY}-W{NN}.md` (weekly) or `.autopus/retro/RETRO-{YYYY}-{MM}.md` (monthly, for periods > 14 days) containing:
- What went well (data-backed)
- What didn't go well (data-backed)
- Action items (derived from patterns)
- Statistics summary

### R7: 특정 SPEC 회고
WHEN the user provides `--spec SPEC-XXX-NNN`, THE SYSTEM SHALL generate a retro focused on that SPEC: related commits, decision trail (Lore entries), test coverage, and timeline from draft to completed.

### R8: 스킬 라우터 등록
WHEN the `/auto` router processes the `retro` subcommand, THE SYSTEM SHALL route to the retro skill handler defined in the auto-router template.

### R9: retro 저장 디렉토리
WHERE retro documents are generated, THE SYSTEM SHALL store them in `.autopus/retro/` directory under the target module root.

## 생성 파일 상세

| 파일/경로 | 역할 |
|-----------|------|
| `templates/claude/commands/auto-retro.md.tmpl` | retro 서브커맨드 스킬 템플릿 |
| `templates/codex/prompts/auto-retro.md.tmpl` | codex 플랫폼 retro 프롬프트 |
| `templates/gemini/commands/auto-retro.md.tmpl` | gemini 플랫폼 retro 커맨드 |
| `auto-router.md.tmpl` (수정) | retro 서브커맨드 라우팅 추가 |
| `.autopus/retro/` (런타임 생성) | 회고 문서 저장 디렉토리 |

## 출력 형식 (RETRO 문서)

```markdown
# RETRO-2026-W14: 주간 회고

**기간**: 2026-03-30 ~ 2026-04-05
**생성일**: 2026-04-05

## 통계
- 커밋: N개 (기여자 M명)
- 파일 변경: X개 (+Y/-Z lines)
- SPEC 완료: K개
- Lore 의사결정: L개

## What Went Well
- [데이터 기반 긍정 패턴]

## What Didn't Go Well
- [데이터 기반 개선 필요 영역]

## Action Items
- [ ] [구체적 액션]
```

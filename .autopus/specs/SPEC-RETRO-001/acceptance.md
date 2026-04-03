# SPEC-RETRO-001 수락 기준

## 시나리오

### S1: 기본 주간 회고 생성
- Given: 최근 7일간 git 커밋이 5개 이상 존재하는 프로젝트
- When: `/auto retro` 실행
- Then: `.autopus/retro/RETRO-2026-W{NN}.md` 파일이 생성되고, 커밋 통계/SPEC 현황/What went well/What didn't/Action items 섹션이 포함됨

### S2: 커스텀 기간 회고
- Given: 2026년 3월의 git 히스토리가 존재
- When: `/auto retro --since 2026-03-01 --until 2026-03-31` 실행
- Then: `.autopus/retro/RETRO-2026-03.md` 월간 형식으로 생성됨 (14일 초과 기간)

### S3: 특정 SPEC 회고
- Given: SPEC-ORCH-001이 completed 상태이고 관련 커밋이 존재
- When: `/auto retro --spec SPEC-ORCH-001` 실행
- Then: SPEC-ORCH-001에 대한 전용 회고가 생성되고, 관련 커밋 목록/의사결정 트레일/소요 기간이 포함됨

### S4: Lore 패턴 분석 포함
- Given: 기간 내 Lore 트레일러가 포함된 커밋이 존재
- When: `/auto retro` 실행
- Then: confidence 분포 (high/medium/low 비율), 자주 거부된 대안 패턴이 분석 결과에 포함됨

### S5: 커밋 없는 기간
- Given: 지정 기간에 커밋이 0개
- When: `/auto retro --since 2025-01-01 --until 2025-01-07` 실행
- Then: "지정 기간에 활동이 없습니다" 메시지가 출력되고, 빈 retro 문서는 생성되지 않음

### S6: 서브커맨드 라우팅
- Given: auto-router가 로드된 상태
- When: `/auto retro` 입력
- Then: retro 스킬 핸들러로 정상 라우팅됨

### S7: 이전 retro 대비 개선 추적
- Given: 이전 주간 retro (`.autopus/retro/RETRO-2026-W13.md`)가 존재
- When: `/auto retro` 실행 (W14)
- Then: 이전 retro의 Action Items 중 완료된 항목이 "What went well"에 반영됨

---
name: frontend-verify
description: 프론트엔드 UX 검증 및 비주얼 테스트 스킬
triggers:
  - verify
  - frontend-verify
  - ux-verify
  - 프론트엔드 검증
  - 비주얼 검증
  - UX 검증
category: quality
level1_metadata: "5-Phase pipeline, VLM visual check, Playwright E2E, self-healing selectors"
---

# Frontend Verify Skill

Claude Vision을 활용하여 프론트엔드 UX를 5단계 파이프라인으로 자동 검증하는 스킬입니다.

## 5단계 파이프라인

### Phase 0: 변경 범위 분석

`git diff`를 기반으로 영향 범위를 파악합니다.

- MVP 범위: React `.tsx` / `.jsx` 파일만 대상
- 변경된 컴포넌트 목록 및 연관 페이지 추출
- 검증 범위를 변경된 페이지로 한정하여 토큰 비용 최소화

```bash
git diff --name-only HEAD~1 | grep -E '\.(tsx|jsx)$'
```

### Phase 1: 테스트 생성 / 셀렉터 자가 치유

변경된 컴포넌트에 대한 Playwright E2E 테스트를 생성하거나 깨진 셀렉터를 복구합니다.

- **신규 컴포넌트**: E2E 테스트 파일 자동 생성
- **깨진 셀렉터**: DOM 스냅샷을 참조하여 셀렉터 자가 치유
  - `aria-label`, `data-testid`, 텍스트 기반 셀렉터 우선 적용
  - CSS 클래스 기반 셀렉터는 최후 수단으로만 사용

### Phase 2: 테스트 실행

Playwright를 실행하고 검증에 필요한 산출물을 수집합니다.

```bash
npx playwright test --reporter=json
```

- 주요 상태(초기 로드, 인터랙션 후, 에러 상태)에서 스크린샷 캡처
- 각 상태의 HTML 스냅샷 저장
- 1x DPR(Device Pixel Ratio) 고정으로 토큰 비용 절감

### Phase 3: VLM 시각 검증

캡처된 스크린샷을 Claude Vision으로 분석합니다.

**판정 기준**

| 판정 | 의미 |
|------|------|
| PASS | 레이아웃 정상, 시각적 문제 없음 |
| WARN | 경미한 문제 감지 (자동 수정 시도 가능) |
| FAIL | 심각한 레이아웃 파괴 또는 접근성 문제 |

**무시 규칙** (판정에서 제외)
- 서브픽셀 단위 폰트 렌더링 차이
- 안티앨리어싱으로 인한 경계 흐림
- 1px 이하의 보더 위치 차이

**WARN/FAIL 판정 조건**
- 콘텐츠 잘림 (클리핑)
- 요소 겹침 (오버랩)
- 텍스트가 보이지 않거나 배경색과 구분 불가
- 반응형 레이아웃 붕괴 (뷰포트 이탈, 가로 스크롤 발생)

**판정 출력 형식**

```markdown
### 스크린샷: [파일명]
판정: PASS / WARN / FAIL
문제: [발견된 시각적 문제 서술 — 수치 점수 없음]
```

### Phase 4: 리포터 및 자동 수정

WARN/FAIL 판정에 대해 수정을 시도하고 최종 보고서를 생성합니다.

- `--fix` 활성화 시: CSS/레이아웃 수정 후 Phase 2-3 재실행
- `--report-only` 모드 시: 수정 없이 보고서만 생성
- 재검증 후에도 FAIL이면 사용자 개입 요청

## CLI 플래그

| 플래그 | 기본값 | 설명 |
|--------|--------|------|
| `--fix` | 활성화 | WARN/FAIL 자동 수정 시도 |
| `--report-only` | 비활성화 | 수정 없이 보고서만 출력 |
| `--viewport <size>` | `1280x800` | 검증 뷰포트 크기 지정 |

## 토큰 비용 최적화

- DPR 1x 고정으로 이미지 해상도 제한
- 변경 범위 내 페이지만 분석 (전체 회귀 검증 금지)
- HTML 스냅샷을 먼저 분석 후 필요 시 스크린샷 투입

## 통합 방식

- **독립 실행**: `/auto verify` 명령으로 단독 실행
- **팀 파이프라인**: `--team` 모드의 Phase 3.5로 선택적 삽입
  - executor 완료 후, reviewer 실행 전에 자동 트리거

## 최종 보고서 형식

```markdown
## Frontend Verify 결과

### 요약
검증 범위: [컴포넌트 목록]
전체 판정: PASS / WARN / FAIL

### 스크린샷별 판정
| 스크린샷 | 판정 | 문제 |
|----------|------|------|

### 수정 내역
- [수정한 파일 및 내용]

### 미해결 문제
- [자동 수정 실패 항목 — 수동 개입 필요]
```

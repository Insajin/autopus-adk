# SPEC-CANARY-001 수락 기준

## 시나리오

### S1: 기본 canary 실행 — 빌드 + E2E
- Given: Go CLI 프로젝트에 `scenarios.md`가 존재하고 active 시나리오가 1개 이상
- When: `/auto canary` 실행
- Then: 빌드 검증 → E2E 시나리오 실행 → PASS/WARN/FAIL 판정이 출력됨

### S2: 빌드 실패 시 FAIL 판정
- Given: 빌드 커맨드가 실패하는 상태
- When: `/auto canary` 실행
- Then: FAIL 판정과 함께 빌드 에러 메시지가 출력되고, E2E/브라우저 검진은 스킵됨

### S3: URL 지정 브라우저 검진
- Given: 실행 중인 웹 애플리케이션 URL
- When: `/auto canary --url http://localhost:3000` 실행
- Then: 해당 URL에 대해 접근성 트리 스냅샷, 콘솔 에러 수집이 수행됨

### S4: watch 모드
- Given: 정상 작동하는 프로젝트
- When: `/auto canary --watch 5m` 실행
- Then: 5분 간격으로 health check가 반복 실행되고, FAIL 발생 시 즉시 중단 및 알림

### S5: 커밋 비교
- Given: `.autopus/canary/{commit-hash}.json`에 이전 canary 결과가 존재
- When: `/auto canary --compare abc123` 실행
- Then: 현재 결과와 지정 커밋 결과의 diff가 표시됨 (새로 실패한 시나리오, 새로 성공한 시나리오)

### S6: sync 후 canary 안내
- Given: `/auto sync` 실행 완료
- When: sync 결과 출력
- Then: "다음: `/auto canary`" 안내가 포함됨

### S7: 서브커맨드 라우팅
- Given: auto-router가 로드된 상태
- When: `/auto canary` 입력
- Then: canary 스킬 핸들러로 정상 라우팅됨

### S8: scenarios.md 없는 프로젝트
- Given: scenarios.md가 존재하지 않는 프로젝트
- When: `/auto canary` 실행
- Then: 빌드 검증만 수행하고 "E2E 시나리오가 없습니다. /auto setup으로 생성하세요" 안내 출력

# SPEC-COMPRESS-001 수락 기준

## 시나리오

### S1: 임계값 미만 — 원문 유지
- Given: 파이프라인 2개 페이즈 완료, 누적 컨텍스트 10K 토큰 (임계값 100K)
- When: executor 페이즈 결과를 tester 페이즈에 전달
- Then: 원문이 변경 없이 그대로 prevOutput에 할당됨

### S2: 임계값 초과 — 구조화 요약 생성
- Given: planner 출력 80K 토큰, executor 출력 70K 토큰 (누적 150K, 임계값 100K)
- When: executor 결과를 tester에 전달하기 전 compressor 호출
- Then: prevOutput이 Goal/Progress/Decisions/Files Modified/Next Steps 섹션을 포함하는 구조화 요약으로 교체됨

### S3: 요약 크기 제한
- Given: 200K context window 모델 사용
- When: 구조화 요약 생성
- Then: 요약 크기가 10K 토큰(5%) 이하

### S4: 도구 결과 가지치기
- Given: executor 출력에 10개의 tool output 포함
- When: pruner가 실행됨
- Then: 첫 번째와 마지막 tool output은 보존되고, 중간 8개는 `[pruned: ...]` placeholder로 교체됨

### S5: 누적 압축 정보 보존
- Given: planner→executor에서 1차 압축 수행, executor→tester에서 2차 압축 수행
- When: tester가 받은 요약 확인
- Then: planner 단계의 주요 결정 사항이 2차 요약의 Decisions 섹션에 보존됨

### S6: Provider별 토큰 예산
- Given: Claude adapter 사용 (200K window)
- When: TokenBudget 계산
- Then: 압축 임계값이 100K (50%)로 설정됨

### S7: Gemini adapter 사용 시
- Given: Gemini adapter 사용 (1M window)
- When: TokenBudget 계산
- Then: 압축 임계값이 500K (50%)로 설정됨

### S8: compressor nil — 기존 동작 유지
- Given: PipelineExecutor에 compressor가 nil로 설정
- When: 파이프라인 실행
- Then: 기존과 동일하게 전문 전달 (하위 호환성 보장)

### S9: 파일 크기 준수
- Given: 모든 신규 소스 파일
- When: 라인 수 확인
- Then: 각 파일이 200줄 이하 (300줄 hard limit 미만)

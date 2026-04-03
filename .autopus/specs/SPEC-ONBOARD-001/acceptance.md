# SPEC-ONBOARD-001 수락 기준

## 시나리오

### S1: install.sh 비지원 OS 에러 메시지
- Given: 사용자가 Windows (WSL 아닌 네이티브) 환경에서 install.sh를 실행
- When: detect_os()가 알 수 없는 OS를 감지
- Then: "현재 macOS와 Linux를 지원합니다. Windows는 WSL2를 통해 사용할 수 있습니다: https://..." 형태의 안내를 표시

### S2: install.sh sudo 설명
- Given: INSTALL_DIR이 쓰기 권한 없는 경로 (/usr/local/bin)
- When: 바이너리 복사 시 sudo가 필요
- Then: sudo 실행 전 "시스템 폴더에 설치하기 위해 관리자 비밀번호가 필요합니다" 메시지를 표시

### S3: install.sh 완료 후 next steps에 worker 포함
- Given: install.sh가 정상 설치 및 init 완료
- When: 최종 안내 메시지 출력
- Then: "auto worker setup" 명령어가 next steps에 포함

### S4: Device Auth 카운트다운 표시
- Given: worker setup에서 Device Auth 단계 진입
- When: 사용자가 브라우저에서 인증을 완료하기 전
- Then: "인증 대기 중... (남은 시간: X분 Y초)" 형태로 카운트다운이 매 폴링 주기마다 갱신

### S5: 브라우저 자동 열기 실패 시 URL 복사 안내
- Given: Device Auth에서 브라우저 자동 열기 시도
- When: OpenBrowser()가 에러를 반환
- Then: "위 URL을 복사하여 브라우저에 붙여넣으세요" 안내를 표시

### S6: PKCE 에러 사용자 메시지
- Given: Device Auth 진행 중
- When: PKCE 관련 에러 (generate PKCE, code_verifier 등) 발생
- Then: "서버 인증 중 오류가 발생했습니다. 잠시 후 다시 시도해주세요." 표시 (기술 용어 미노출)

### S7: 미설치 프로바이더 단계별 안내
- Given: worker setup Step 3에서 프로바이더 상태 확인
- When: claude가 미설치
- Then: "1. https://claude.ai 에서 가입 → 2. npm install -g @anthropic-ai/claude-code → 3. claude login 실행" 형태의 단계별 안내 표시

### S8: 환경변수 설정 안내 (비개발자 친화)
- Given: codex/opencode 프로바이더의 인증 필요
- When: CheckProviderAuth()가 가이드를 반환
- Then: "Set OPENAI_API_KEY" 대신 "터미널에 다음 명령어를 입력하세요: export OPENAI_API_KEY=여기에_키_입력" 표시

### S9: init 후 올바른 next step
- Given: auto init이 성공적으로 완료
- When: 요약 화면 출력
- Then: "auto doctor" 대신 컨텍스트에 맞는 다음 단계 안내 (worker setup 필요 시 "auto worker setup", 아니면 "/auto plan")

### S10: Worker 개념 설명
- Given: auto worker setup 명령어 실행
- When: 셋업 위저드 시작
- Then: "Worker는 Autopus 서버에서 작업을 받아 자동으로 실행하는 백그라운드 서비스입니다" 설명이 위저드 상단에 표시

### S11: 에러 체인 사용자 메시지 변환
- Given: worker setup 중 HTTP 500 에러 발생
- When: 에러가 사용자에게 표시
- Then: "authentication failed: request device code: HTTP 500" 대신 "Autopus 서버에 연결할 수 없습니다. 인터넷 연결을 확인하고 다시 시도해주세요." 표시

### S12: 파일 크기 제한 준수
- Given: 모든 변경 완료
- When: 소스코드 파일 라인 수 확인
- Then: 모든 .go 파일이 300줄 이하

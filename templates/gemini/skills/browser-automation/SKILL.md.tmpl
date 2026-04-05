---
name: auto-browser-automation
---

# Browser Automation Skill

터미널 환경을 자동 감지하여 cmux browser 또는 agent-browser CLI를 활용해 AI 에이전트가 직접 웹 페이지를 조작하고 검증하는 스킬입니다.

## 도구 선택

| 도구 | 용도 | 선택 기준 |
|------|------|-----------|
| **cmux browser** | cmux 환경 네이티브 조작 | cmux 터미널에서 자동 선택 (최우선) |
| **agent-browser** | 에이전트 직접 조작 | tmux/plain 환경 폴백 |
| **Playwright** | E2E 테스트 스위트 | 반복 실행, CI/CD, 회귀 테스트 |

백엔드는 `auto terminal detect` 결과에 따라 자동 선택됩니다. 수동 지정하지 마세요.

## 백엔드 자동 감지

브라우저 백엔드는 현재 터미널 환경에 따라 자동 선택됩니다.

**감지 방법:**
```bash
auto terminal detect
```

| 터미널 | 백엔드 | 명령어 접두사 |
|--------|--------|---------------|
| **cmux** | cmux browser | `cmux browser <subcommand>` |
| **tmux / plain** | agent-browser | `agent-browser <subcommand>` |

IMPORTANT: 스킬 실행 시 반드시 `auto terminal detect`를 먼저 호출하여 백엔드를 결정해야 합니다. agent-browser를 하드코딩하지 마세요.

### 명령어 매핑

| 동작 | cmux browser | agent-browser |
|------|-------------|---------------|
| 페이지 열기 | `cmux browser open <url>` | `agent-browser open <url>` |
| 스냅샷 | `cmux browser --surface <ref> snapshot` | `agent-browser snapshot` |
| 클릭 | `cmux browser --surface <ref> click --selector <sel>` | `agent-browser click <ref>` |
| 입력 | `cmux browser --surface <ref> fill --selector <sel> --text <text>` | `agent-browser fill <ref> <text>` |
| 스크린샷 | `cmux browser --surface <ref> screenshot --out <path>` | `agent-browser screenshot <path>` |
| 대기 | `cmux browser --surface <ref> wait --text <text>` | `agent-browser wait --text <text>` |
| URL 확인 | `cmux browser --surface <ref> get url` | `agent-browser get url` |
| 텍스트 확인 | `cmux browser --surface <ref> get text --selector <sel>` | `agent-browser get text <ref>` |
| 표시 여부 | `cmux browser --surface <ref> is visible --selector <sel>` | `agent-browser is visible <ref>` |

**cmux 고유 기능:**
- `--surface <ref>`: cmux browser는 surface 핸들로 세션을 관리합니다. `open` 명령이 반환하는 surface ref를 후속 명령에 사용합니다.
- `--snapshot-after`: 클릭/입력 후 자동 스냅샷 (`cmux browser --surface <ref> click --selector <sel> --snapshot-after`)
- `--interactive`: 인터랙티브 스냅샷 모드 (`cmux browser --surface <ref> snapshot --interactive`)

## 사전 조건

```bash
# 터미널 감지
auto terminal detect

# cmux 환경 (자동 감지)
cmux --version

# agent-browser 환경 (폴백)
agent-browser --version
# 미설치 시
npm install -g agent-browser
agent-browser install
```

## 핵심 워크플로우: Snapshot-Act-Verify

AI 에이전트가 브라우저를 조작하는 단계별 루프:

### Step 0: Detect Backend — 터미널 감지

```bash
BACKEND=$(auto terminal detect)
# "cmux" → cmux browser 사용
# "tmux" or "plain" → agent-browser 사용
```

### Step 1: Open — 페이지 열기

**cmux:**
```bash
cmux browser open <url>
# Returns: surface reference (e.g., "surface:1")
```

**agent-browser:**
```bash
agent-browser open <url>
```

### Step 2: Snapshot — 접근성 트리 + 참조 획득

**cmux:**
```bash
cmux browser --surface <ref> snapshot
```

**agent-browser:**
```bash
agent-browser snapshot
```

snapshot은 페이지의 접근성 트리를 반환하며, 각 요소에 `@e1`, `@e2` 등의 참조를 할당한다. AI 에이전트는 이 참조를 사용하여 요소를 조작한다.

**snapshot 출력 예시:**
```
- @e1 heading "AI Settings"
- @e2 button "Provider Mode"
- @e3 switch "Auto Fallback" [checked]
- @e4 checkbox "Anthropic" [checked]
- @e5 checkbox "OpenAI" [checked]
- @e6 checkbox "Google" [unchecked]
- @e7 button "Save"
```

### Step 3: Act — 요소 상호작용

**cmux:**
```bash
cmux browser --surface <ref> click --selector "@e3"
cmux browser --surface <ref> fill --selector "@e4" --text "text"
cmux browser --surface <ref> press Enter
```

**agent-browser:**
```bash
agent-browser click @e3
agent-browser fill @e4 "text"
agent-browser press Enter
```

### Step 4: Verify — 상태 확인 + 스크린샷

**cmux:**
```bash
cmux browser --surface <ref> snapshot
cmux browser --surface <ref> screenshot --out /tmp/verify.png
cmux browser --surface <ref> is visible --selector "@e3"
cmux browser --surface <ref> is checked --selector "@e3"
cmux browser --surface <ref> get text --selector "@e1"
```

**agent-browser:**
```bash
agent-browser snapshot
agent-browser screenshot /tmp/verify.png
agent-browser is visible @e3
agent-browser is checked @e3
agent-browser get text @e1
```

## 주요 명령어 레퍼런스

### 네비게이션

**cmux:**
```bash
cmux browser open <url>
cmux browser --surface <ref> get url
cmux browser --surface <ref> get title
cmux browser --surface <ref> wait --load-state complete
cmux browser --surface <ref> wait --text "Welcome"
```

**agent-browser:**
```bash
agent-browser open <url>
agent-browser get url
agent-browser get title
agent-browser wait --load networkidle
agent-browser wait --text "Welcome"
```

### 상호작용

**cmux:**
```bash
cmux browser --surface <ref> click --selector <sel>
cmux browser --surface <ref> fill --selector <sel> --text <text>
cmux browser --surface <ref> hover --selector <sel>
cmux browser --surface <ref> scroll down 500
cmux browser --surface <ref> press Enter
```

**agent-browser:**
```bash
agent-browser click <ref>
agent-browser fill <ref> <text>
agent-browser type <ref> <text>
agent-browser hover <ref>
agent-browser scroll down 500
agent-browser press Enter
```

### 의미론적 로케이터 (snapshot 없이 직접 찾기)

**cmux:**
```bash
cmux browser --surface <ref> find role button --name "Save"
cmux browser --surface <ref> find text "Sign In"
cmux browser --surface <ref> find label "Email"
```

**agent-browser:**
```bash
agent-browser find role button click --name "Save"
agent-browser find text "Sign In" click
agent-browser find label "Email" fill "test@test.com"
```

### 상태 확인

**cmux:**
```bash
cmux browser --surface <ref> is visible --selector <sel>
cmux browser --surface <ref> is enabled --selector <sel>
cmux browser --surface <ref> is checked --selector <sel>
cmux browser --surface <ref> get text --selector <sel>
cmux browser --surface <ref> get html --selector <sel>
```

**agent-browser:**
```bash
agent-browser is visible <ref>
agent-browser is enabled <ref>
agent-browser is checked <ref>
agent-browser get text <ref>
agent-browser get html <ref>
```

### 뷰포트 & 디바이스

**cmux:**
```bash
cmux browser --surface <ref> viewport 1280 800
```

**agent-browser:**
```bash
agent-browser set viewport 1280 800
agent-browser set device "iPhone 14"
agent-browser set media dark
```

### 쿠키 & 인증

**cmux:**
```bash
cmux browser --surface <ref> cookies get
cmux browser --surface <ref> cookies set --name <name> --value <value>
cmux browser --surface <ref> storage local get
```

**agent-browser:**
```bash
agent-browser cookies
agent-browser cookies set <name> <value>
agent-browser storage local
```

## 실행 모드

| 백엔드 | 기본 모드 | 설명 |
|--------|-----------|------|
| **cmux browser** | 내장 서피스 | cmux 터미널 내 브라우저 패널로 표시. `--headed` 불필요 |
| **agent-browser** (기본) | Headless | `--headed` 추가 시 브라우저 창 표시 |

```bash
# cmux — 항상 터미널 내 브라우저 패널로 표시
cmux browser open https://autopus.co

# agent-browser — 헤드리스 (기본)
agent-browser open https://autopus.co
# agent-browser — 브라우저 창 표시
agent-browser open https://autopus.co --headed
```

## 운영환경 검증 패턴

> 아래 예시는 cmux 백엔드 기준입니다. agent-browser 사용 시 `cmux browser --surface <ref>` 대신 `agent-browser`를 사용하세요.

### 패턴 1: UI 컴포넌트 존재 확인

```bash
SURFACE=$(cmux browser open https://example.com/settings/ai)
cmux browser --surface $SURFACE snapshot
# → @e3 switch "Auto Fallback" 이 존재하면 렌더링 성공
cmux browser --surface $SURFACE screenshot --out /tmp/ai-settings.png
```

### 패턴 2: 토글 동작 검증

```bash
cmux browser --surface $SURFACE snapshot
cmux browser --surface $SURFACE click --selector "@e3" --snapshot-after
```

### 패턴 3: 인증 후 테스트

```bash
SURFACE=$(cmux browser open https://example.com/login)
cmux browser --surface $SURFACE snapshot
cmux browser --surface $SURFACE fill --selector "@e2" --text "user@example.com"
cmux browser --surface $SURFACE fill --selector "@e3" --text "password"
cmux browser --surface $SURFACE click --selector "@e4"
cmux browser --surface $SURFACE wait --load-state complete
cmux browser --surface $SURFACE goto https://example.com/settings/ai
cmux browser --surface $SURFACE snapshot
```

## 판정 기준

| 판정 | 기준 |
|------|------|
| PASS | 기대 요소가 snapshot에 존재하고 올바른 상태 |
| WARN | 요소는 존재하나 상태가 예상과 다름 |
| FAIL | 기대 요소가 snapshot에 없거나 에러 발생 |

## 주의사항

- 백엔드는 `auto terminal detect`로 자동 선택됨 — 수동 지정 금지
- cmux 환경에서는 `cmux browser`가 최우선 — agent-browser를 사용하면 세션이 분리됨
- cmux `--surface` 핸들은 `open` 명령이 반환 — 후속 명령에 반드시 전달
- `snapshot`은 **접근성 트리**만 반환 — CSS 스타일은 포함 안 됨 (시각 확인은 `screenshot` 사용)
- 운영환경 테스트 시 **쓰기 작업**(삭제, 설정 변경)은 신중하게 — 되돌릴 수 없을 수 있음

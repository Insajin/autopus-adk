---
name: monitor-patterns
description: Monitor tool usage patterns and grep --line-buffered compatibility guide
triggers:
  - monitor
  - monitor-patterns
  - line-buffered
category: agentic
level1_metadata: "Monitor tool usage, grep line-buffered, idle prompt regex, orchestra Round 2 wait"
---

# Monitor Patterns Skill

Claude Code 2.1.x `Monitor` tool을 활용한 pane 이벤트 스트리밍 패턴.
주 사용처는 orchestra Round 2 응답 대기(`idea.md` Step 3.6).

## 1. Monitor Tool 개요

`Monitor`는 실행 중인 command의 stdout을 실시간 tail하여 각 라인을 notification으로
전달한다. `until-loop`/`polling sleep` 보다 저렴하다 — 대기 중 토큰 소비 없음.

기본 호출:

```python
Monitor(
  command = "...",
  timeout_ms = 180000
)
```

반환값: notification stream (stdout의 각 라인이 1개 이벤트).

**활성화 조건**: `AUTOPUS_PLATFORM == claude-code` AND `features.cc21.monitor_enabled: true`.
비 Claude Code CLI 환경에서는 polling fallback으로 graceful degradation (R8).

## 2. grep --line-buffered 요구사항

Monitor가 실시간으로 라인을 수신하려면 command가 line-buffered 모드로 출력해야 한다.
stdout이 pipe 앞단이면 기본값은 fully-buffered — 이벤트가 버퍼 flush 전까지 지연된다.

### 올바른 패턴

```bash
# GNU grep (Linux 기본값, macOS Homebrew grep)
grep --line-buffered -E 'pattern'

# BSD grep (macOS 기본값) — stdbuf로 강제 line-buffering
stdbuf -oL grep -E 'pattern'

# Universal fallback — unbuffer (expect 패키지)
unbuffer grep -E 'pattern'
```

### 잘못된 패턴 — Monitor가 실시간 이벤트를 받지 못함

```bash
grep -E 'pattern'          # fully-buffered 기본값 — 이벤트 지연
cat file | grep 'pattern'  # grep 자체도 line-buffered 필요
```

## 3. 플랫폼 감지 및 분기 템플릿

```bash
# Detect GNU vs BSD grep and set GREP_LB accordingly
if grep --version 2>/dev/null | grep -qi "gnu grep"; then
  GREP_LB="grep --line-buffered"
else
  GREP_LB="stdbuf -oL grep"
fi
```

Monitor 호출 시 적용:

```python
Monitor(
  command = f"cmux read-screen --surface {surface_id} --follow --scrollback 200 | {GREP_LB} -E '{idle_regex}'",
  timeout_ms = 180000
)
```

## 4. Provider별 idle prompt 정규식

`spec.md R7` 정의. provider pane이 응답을 마치고 입력 대기 상태에 진입하면
해당 라인이 stdout으로 출력된다.

| Provider | Pattern |
|----------|---------|
| claude | `^❯\s*$` |
| codex | `^codex>\s*$` |
| gemini-cli | `^>\s*(Type your\|Press Ctrl)` 또는 `gemini>\s*$` |
| opencode | `^opencode›\s*$` 또는 `^>\s*Ready\s*$` |

### 오버라이드

`.autopus/project/orchestra-patterns.yaml`:

```yaml
orchestra_patterns:
  claude:
    idle: '^❯\s*$'
  codex:
    idle: '^codex>\s*$'
  gemini-cli:
    idle: '^>\s*(Type your|Press Ctrl)'
  opencode:
    idle: '^opencode›\s*$'
```

## 5. Orchestra Round 2 대기 — 전체 예시

`idea.md` Step 3.6 구현 레퍼런스.

```python
# Step 0: resolve GREP_LB at runtime (see §3)
providers = ["claude", "codex", "gemini-cli"]
pending = set(providers)

# Step 1: launch Monitor per provider (parallel)
for provider in providers:
    surface = get_surface_id(provider)           # cmux surface lookup
    pattern = get_idle_pattern(provider)         # from orchestra-patterns.yaml or defaults
    Monitor(
      command = f"cmux read-screen --surface {surface} --follow --scrollback 200 | {GREP_LB} -E '{pattern}'",
      timeout_ms = 180000
    )

# Step 2: process notifications as they arrive
#   On each notification:
#     provider_name = identify_provider(notification.source)
#     pending.discard(provider_name)
#     if not pending:
#         break  # all providers idle

# Step 3: collect scrollback once all panes are done
for provider in providers:
    surface = get_surface_id(provider)
    result = bash(f"cmux read-screen --surface {surface} --scrollback --lines 500")
    store_round2_result(provider, result)
```

## 6. Timeout 처리 및 Fallback

Monitor `timeout_ms` 경과 시 해당 provider만 polling fallback으로 전환한다.
다른 provider의 Monitor는 유지한다.

```python
# On Monitor timeout for a specific provider:
log(f"monitor_timeout=true, fallback=polling, provider={provider}")
bash(f"sleep 120")
result = bash(f"cmux read-screen --surface {surface} --scrollback --lines 500")
store_round2_result(provider, result)
```

**비 Claude Code 환경 전체 fallback** (`AUTOPUS_PLATFORM != claude-code`):

```bash
sleep 120  # 2-minute polling wait
cmux read-screen --surface {surface} --scrollback --lines 500
```

## 7. Anti-Patterns

- **grep `--line-buffered` 누락** → pipe 버퍼링으로 Monitor notification 지연 또는 미수신.
  macOS BSD grep에서 특히 발생.
- **Monitor timeout 너무 짧게 설정**(`< 30s`) → 응답 생성 중 false timeout. 최솟값 60s 권장.
- **단일 Monitor에 여러 pane 멀티플렉스 시도** → notification 출처 구분 불가.
  provider당 1개 Monitor 호출 원칙.
- **bash special char 미escape** (`|`, `(`, `)` 등을 grep pattern 내 그대로 사용)
  → 쉘 파싱 오류 또는 매칭 실패. f-string/template 삽입 시 escape 확인 필수.
- **Round 2 수집 전 Step 4 진입** → `idea.md` HARD BLOCK 조건. Monitor가 완료 신호를
  주기 전에 scrollback을 수집하면 응답이 잘린다.

## 8. auto check 연동

`auto check --monitor-commands` lint는 Monitor command 문자열을 스캔하여
`--line-buffered` / `stdbuf -oL` / `unbuffer` 중 하나가 없으면 warning을 발행한다
(acceptance.md S7-3).

경고 예시:

```
WARN  monitor-patterns  command lacks line-buffered guard: "cmux read-screen ... | grep -E '...'"
      → add --line-buffered (GNU) or stdbuf -oL (BSD)
```

## 9. 성능 지표

`spec.md §5` 참조. Monitor notification 지연 SLO:

| 지표 | 목표 |
|------|------|
| p95 notification latency | ≤ 500ms |
| 측정 기준 | idle prompt 라인 stdout 기록 timestamp ↔ Monitor notification 수신 timestamp |
| 기록 위치 | `logs/pipeline/<run-id>.jsonl` (`monitor_notifications` 필드) |

## Ref

- `spec.md` R7 — Monitor 기반 Round 2 대기 요구사항 및 provider별 idle 패턴
- `idea.md` Step 3.6 — Monitor 기반 Round 2 결과 수집 절차
- BSD grep man page — `--line-buffered` 미지원 확인
- Claude Code 2.1.x Monitor tool 공식 문서
- `acceptance.md` S7-3 — auto check --monitor-commands lint 검증 케이스

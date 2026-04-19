# .claude/hooks — 운영 가이드

Claude Code 2.1.x `hooks.TaskCreated` 배열에 등록되는 훅 스크립트 모음.

---

## task-created-validate.sh

TodoWrite UI 인터랙션 시 제목 품질을 자동 검증한다 (SPEC-CC21-001 R5·R6).

### 환경변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `TASKCREATED_MODE` | unset | 런타임 override. `warn` 또는 `enforce` |
| `AUTOPUS_TASKCREATED_DEFAULT_MODE` | `warn` | generated default. `autopus.yaml features.cc21.task_created_mode`가 이 값으로 내려온다 |
| `AUTOPUS_TASK_AUDIT_LOG` | `.autopus/task-audit.log` | 감사 로그 파일 경로 |

### 검증 규칙

1. **제목 길이** — 5자 이상이어야 한다.
2. **Action verb** — 제목의 첫 단어가 아래 allow-list에 있어야 한다.

```text
add update remove fix refactor rename move rewrite implement create
delete clean check verify document migrate split merge extract
simplify optimize
```

### stdin 계약

공식 Claude Code payload 키를 우선 사용한다:

- `task_subject`
- `task_description`

로컬 회귀 호환을 위해 현재 스크립트는 legacy 키 `title` / `description`도 fallback으로 허용한다.

### 로그 포맷

감사 로그는 JSON Lines (`.autopus/task-audit.log`) 로 append된다.

예시:

```json
{"timestamp":"2026-04-18T09:00:00Z","outcome":"pass","mode":"warn","reason":null,"title":"add auth flow","spec_id":"SPEC-CC21-001","task_id":null}
{"timestamp":"2026-04-18T09:01:00Z","outcome":"warn","mode":"warn","reason":"title_too_short","title":"fix","spec_id":null,"task_id":null}
{"timestamp":"2026-04-18T09:02:00Z","outcome":"fail","mode":"enforce","reason":"action_verb_not_allowed","title":"quickfix thing","spec_id":null,"task_id":null}
```

### warn → enforce 전환 절차

1. warn 모드로 최소 **3주** 운영한다.
2. 오탐률(WARN 중 실제 유효 제목) < 5% 확인 후 승격한다.
3. 영구 기본값은 `autopus.yaml features.cc21.task_created_mode: enforce`로 올리고 `auto update`를 실행한다.
4. 단발 override는 shell profile 또는 실행 환경에 `TASKCREATED_MODE=enforce`를 설정한다.
5. 전환 날짜와 오탐률 수치를 `CHANGELOG.md`에 기록한다.
6. 회귀 발생 시 즉시 `warn`으로 복귀하고 CHANGELOG에 기록한다.

### jq 설치 안내

`jq`가 없으면 스크립트는 **fail-open**(exit 0)으로 동작하며 stderr에 경고를 출력한다.

```bash
# macOS
brew install jq

# Ubuntu / Debian
sudo apt-get install -y jq

# Alpine
apk add jq
```

### QG1.5 — 실행 권한 체크 스크립트

커밋 전 아래 명령으로 실행 권한을 확인한다:

```bash
#!/usr/bin/env bash
# Check executable bit for all hooks
FAIL=0
for f in .claude/hooks/*.sh; do
  if [[ ! -x "$f" ]]; then
    echo "[ERROR] not executable: $f" >&2
    FAIL=1
  fi
done
[[ $FAIL -eq 0 ]] && echo "[OK] all hooks are executable"
exit $FAIL
```

`chmod +x .claude/hooks/task-created-validate.sh` 로 권한을 부여하고 커밋한다.

---

## .claude/settings.json 등록 예시

```json
{
  "hooks": {
    "TaskCreated": [
      {
        "command": "bash .claude/hooks/task-created-validate.sh"
      }
    ]
  }
}
```

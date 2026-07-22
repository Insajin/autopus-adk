#!/bin/sh

VERSION_SMOKE_TIMEOUT_SECONDS=15
VERSION_SMOKE_FILE_BLOCKS=2
# shellcheck disable=SC2034 # Sourced by install.sh.
DOCTOR_TIMEOUT_SECONDS=300
PROCESS_TERMINATION_GRACE_SECONDS=2

process_identity() {
    identity_value="$(ps -o pgid=,lstart= -p "$1" 2>/dev/null | awk '{$1=$1; print}')"
    if [ -n "$identity_value" ]; then
        printf '%s\n' "$identity_value"
        return
    fi
    if [ -r "/proc/$1/stat" ]; then
        proc_stat="$(cat "/proc/$1/stat" 2>/dev/null)" || return
        proc_rest="${proc_stat##*) }"
        # shellcheck disable=SC2086 # /proc fields require intentional splitting.
        set -- $proc_rest
        [ "$#" -ge 20 ] && printf '%s %s proc\n' "$3" "${20}"
    fi
}

# Snapshot descendants attached at the deadline. Intentionally detached daemons are out of scope.
capture_process_snapshot() {
    snapshot_root="$1" snapshot_file="$2"
    snapshot_pids="$(ps -eo pid=,ppid= 2>/dev/null | awk -v root="$snapshot_root" '
        { parent[$1]=$2 }
        END { for (pid in parent) { cursor=pid; depth=0
            while (cursor != root && cursor in parent && depth++ < 1024) cursor=parent[cursor]
            if (cursor == root) print pid
        }}')"
    : > "$snapshot_file"
    for snapshot_pid in $snapshot_pids "$snapshot_root"; do
        snapshot_identity="$(process_identity "$snapshot_pid")"
        [ -n "$snapshot_identity" ] && printf '%s\t%s\n' "$snapshot_pid" "$snapshot_identity" >> "$snapshot_file"
    done
}

# Revalidate launch identity before every signal so a reused PID is never targeted.
signal_process_snapshot() {
    signal_file="$1" signal_name="$2"
    while IFS="$(printf '\t')" read -r signal_pid expected_identity; do
        case "$signal_pid" in ''|*[!0-9]*) continue ;; esac
        current_identity="$(process_identity "$signal_pid")"
        [ -n "$current_identity" ] && [ "$current_identity" = "$expected_identity" ] || continue
        if [ "$signal_name" = KILL ]; then
            kill -KILL "$signal_pid" 2>/dev/null || true
        else
            kill -TERM "$signal_pid" 2>/dev/null || true
        fi
    done < "$signal_file"
}

cleanup_bounded_state() {
    rm -f "$1/timeout" "$1/ready" "$1/done" "$1/status" "$1/launch" "$1/processes" "$1/watchdog"
    rmdir "$1" 2>/dev/null || true
}

# Return 124 after deadline cleanup, or 125 if the local supervisor cannot start safely.
run_bounded_command() {
    bounded_timeout_seconds="$1"
    shift
    case "$bounded_timeout_seconds:$PROCESS_TERMINATION_GRACE_SECONDS" in
        *[!0-9:]*|0:*|:*|*:) return 125 ;;
    esac
    [ "$bounded_timeout_seconds" -gt 0 ] || return 125
    [ -n "$(process_identity "$$")" ] || return 125
    bounded_state_dir="$(mktemp -d "${TMPDIR:-/tmp}/autopus-command.XXXXXX")" || return 125
    bounded_timeout_marker="$bounded_state_dir/timeout"
    bounded_ready_marker="$bounded_state_dir/ready"
    bounded_done_marker="$bounded_state_dir/done"
    bounded_status_file="$bounded_state_dir/status"
    bounded_launch_snapshot="$bounded_state_dir/launch"
    bounded_process_snapshot="$bounded_state_dir/processes"
    bounded_watchdog_snapshot="$bounded_state_dir/watchdog"
    (
        set +e
        while [ ! -f "$bounded_ready_marker" ]; do :; done
        "$@" &
        payload_pid=$!
        payload_identity="$(process_identity "$payload_pid")"
        [ -n "$payload_identity" ] && printf '%s\t%s\n' "$payload_pid" "$payload_identity" >> "$bounded_launch_snapshot"
        wait "$payload_pid"
        wrapper_status=$?
        printf '%s\n' "$wrapper_status" > "$bounded_status_file"
        : > "$bounded_done_marker"
        exit "$wrapper_status"
    ) &
    bounded_command_pid=$!
    bounded_launch_identity="$(process_identity "$bounded_command_pid")"
    if [ -z "$bounded_launch_identity" ]; then
        kill -TERM "$bounded_command_pid" 2>/dev/null || true
        kill -KILL "$bounded_command_pid" 2>/dev/null || true
        wait "$bounded_command_pid" 2>/dev/null || true
        cleanup_bounded_state "$bounded_state_dir"
        return 125
    fi
    printf '%s\t%s\n' "$bounded_command_pid" "$bounded_launch_identity" > "$bounded_launch_snapshot"
    : > "$bounded_ready_marker"
    (
        sleep "$bounded_timeout_seconds"
        bounded_current_identity="$(process_identity "$bounded_command_pid")"
        if [ ! -f "$bounded_done_marker" ] &&
            { [ -z "$bounded_current_identity" ] || [ "$bounded_current_identity" = "$bounded_launch_identity" ]; }; then
            : > "$bounded_timeout_marker"
            if [ -z "$bounded_current_identity" ]; then
                cp "$bounded_launch_snapshot" "$bounded_process_snapshot"
                signal_process_snapshot "$bounded_process_snapshot" TERM
                sleep "$PROCESS_TERMINATION_GRACE_SECONDS"
                signal_process_snapshot "$bounded_process_snapshot" KILL
            else
                capture_process_snapshot "$bounded_command_pid" "$bounded_process_snapshot"
                cat "$bounded_launch_snapshot" >> "$bounded_process_snapshot"
                signal_process_snapshot "$bounded_process_snapshot" TERM
                sleep "$PROCESS_TERMINATION_GRACE_SECONDS"
                signal_process_snapshot "$bounded_process_snapshot" KILL
            fi
        fi
    ) &
    bounded_watchdog_pid=$!
    bounded_command_status=0
    wait "$bounded_command_pid" || bounded_command_status=$?
    if [ -f "$bounded_timeout_marker" ]; then
        wait "$bounded_watchdog_pid" 2>/dev/null || true
        bounded_command_status=124
    else
        capture_process_snapshot "$bounded_watchdog_pid" "$bounded_watchdog_snapshot"
        signal_process_snapshot "$bounded_watchdog_snapshot" TERM
        signal_process_snapshot "$bounded_watchdog_snapshot" KILL
        wait "$bounded_watchdog_pid" 2>/dev/null || true
    fi
    cleanup_bounded_state "$bounded_state_dir"
    return "$bounded_command_status"
}

run_version_smoke() {
    version_binary="$1" version_output="$2"
    # shellcheck disable=SC2016 # Positional parameters expand in the child shell.
    run_bounded_command "$VERSION_SMOKE_TIMEOUT_SECONDS" /bin/sh -c \
        'output=$2; ulimit -f "$1" || exit 125; shift 2; exec "$@" > "$output" 2>&1' \
        autopus-version-smoke "$VERSION_SMOKE_FILE_BLOCKS" "$version_output" "$version_binary" version --short
}

version_smoke_output_matches() {
    expected_output_file="$2.expected"
    printf '%s\n' "$1" > "$expected_output_file"
    version_output_status=0
    cmp -s "$expected_output_file" "$2" || version_output_status=1
    rm -f "$expected_output_file"
    return "$version_output_status"
}

print_macos_execution_admission_recovery() {
    echo "  macOS 실행 보안 심사가 제한 시간 안에 완료되지 않았습니다." >&2
    echo "  시스템 설정 > 개인정보 보호 및 보안에서 차단 알림을 확인하세요." >&2
    echo "  차단 알림이 없거나 보안 심사가 계속 멈추면 Mac을 재시동한 뒤 다음 명령을 다시 실행하세요:" >&2
    echo "    ${INSTALL_DIR}/${BINARY} version --short" >&2
    echo "  설치 프로그램은 Gatekeeper를 비활성화하거나 실행 승인을 우회하지 않습니다." >&2
}

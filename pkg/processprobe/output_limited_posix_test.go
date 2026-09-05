//go:build darwin || linux

package processprobe

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputLimited_SuccessReturnsCapturedOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := OutputLimited(exec.CommandContext(ctx, "/bin/sh", "-c", "printf 1.2.3"), 64)

	require.NoError(t, err)
	assert.Equal(t, "1.2.3", string(out))
}

func TestOutputLimited_StreamOverflowTerminatesProcessGroup(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			script := filepath.Join(t.TempDir(), "probe")
			require.NoError(t, os.WriteFile(script, []byte(`#!/bin/sh
(/bin/sleep 30) &
printf '%s' "$!" > "$AUTOPUS_OUTPUT_LIMIT_PID_FILE"
if [ "$1" = stdout ]; then
  while :; do printf '0123456789abcdef0123456789abcdef'; done
else
  while :; do printf '0123456789abcdef0123456789abcdef' >&2; done
fi
`), 0o755))
			// 컨텍스트는 "제한이 아니라 컨텍스트로 반환했다"는 대안 가설을
			// 구별할 수 있을 만큼만 멀리 둔다. 유예의 4배.
			ctx, cancel := context.WithTimeout(context.Background(), 4*DefaultWaitDelay)
			defer cancel()
			// 스크립트를 직접 exec하지 않고 /bin/sh에 인자로 넘긴다. macOS는
			// 처음 실행되는 실행 파일마다 최초 실행 평가(syspolicyd/XProtect)를
			// 시스템 전역으로 직렬화해 파일당 약 190ms를 물리는데, `go test ./...`
			// 아래에서는 다른 패키지의 새 fixture와 같은 큐에 서서 그 대기가
			// `started` 이후 측정창에 들어가 아래 2배 상한을 넘겼다(K=24 동시
			// 실행에서 라운드 4/4 red). 이미 평가된 /bin/sh를 exec하면 그 비용이
			// 사라지고 프로세스 그룹·상속 파이프 계약은 그대로 검증된다.
			cmd := exec.CommandContext(ctx, "/bin/sh", script, stream)
			cmd.Env = append(os.Environ(), "AUTOPUS_OUTPUT_LIMIT_PID_FILE="+pidFile)
			started := time.Now()

			out, err := OutputLimited(cmd, 64)

			assert.ErrorIs(t, err, ErrOutputLimit)
			assert.LessOrEqual(t, len(out), 64)
			// 계약은 두 가지다: 제한에 걸려 반환했다는 것(컨텍스트를 기다리지
			// 않았다), 그리고 상속된 파이프 대기가 유한하다는 것. 두 상한을
			// 모두 소유 상수에서 유도하고 2배 간격으로 벌려, 유예로 반환한
			// 실행(약 1배)과 컨텍스트로 반환한 실행(4배)이 부하에서도 섞이지
			// 않게 한다. 3초 리터럴은 옛 250ms에 여유를 더해 굳힌 값이라
			// 유예가 움직이는 순간 계약과 무관하게 깨졌다.
			assert.Less(t, time.Since(started), 2*DefaultWaitDelay)
			childPID := readPOSIXProbePID(t, pidFile)
			require.Eventually(t, func() bool {
				return errors.Is(syscall.Kill(childPID, 0), syscall.ESRCH)
			}, 2*time.Second, 20*time.Millisecond)
		})
	}
}

func TestOutputLimited_RejectsInvalidLimitWithoutStarting(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	cmd := exec.Command("/bin/sh", "-c", "printf started > \"$AUTOPUS_OUTPUT_LIMIT_MARKER\"")
	cmd.Env = append(os.Environ(), "AUTOPUS_OUTPUT_LIMIT_MARKER="+marker)

	_, err := OutputLimited(cmd, 0)

	require.Error(t, err)
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func readPOSIXProbePID(t *testing.T, path string) int {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, time.Second, 10*time.Millisecond)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	return pid
}

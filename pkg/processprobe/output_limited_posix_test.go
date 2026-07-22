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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, script, stream)
			cmd.Env = append(os.Environ(), "AUTOPUS_OUTPUT_LIMIT_PID_FILE="+pidFile)
			started := time.Now()

			out, err := OutputLimited(cmd, 64)

			assert.ErrorIs(t, err, ErrOutputLimit)
			assert.LessOrEqual(t, len(out), 64)
			assert.Less(t, time.Since(started), 3*time.Second)
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

//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package omp

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPReadinessBehavior_TimeoutTerminatesDescendantProcessGroup(t *testing.T) {
	pidPath := t.TempDir() + "/descendant.pid"
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	report := ProbeOMPReadiness(ctx, behavioralFixtureOptions(t, "timeout-child", "--fixture-pid="+pidPath))

	assert.Equal(t, "timeout", capabilityByID(t, report, "rpc.terminal").Reason)
	var data []byte
	require.Eventually(t, func() bool {
		var err error
		data, err = os.ReadFile(pidPath)
		return err == nil
	}, time.Second, 10*time.Millisecond)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
	}, 2*time.Second, 20*time.Millisecond)
}

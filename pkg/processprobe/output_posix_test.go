//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package processprobe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputSuccessDoesNotTerminateProcessGroup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "completed")
	t.Setenv("AUTOPUS_TEST_SUCCESS_MARKER", marker)
	script := filepath.Join(t.TempDir(), "probe")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/sh
(
  exec >/dev/null 2>&1
  /bin/sleep 0.2
  printf done > "$AUTOPUS_TEST_SUCCESS_MARKER"
) &
printf '1.2.3\n'
`), 0o755))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := Output(exec.CommandContext(ctx, script))

	require.NoError(t, err)
	assert.Equal(t, "1.2.3\n", string(out))
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(marker)
		return statErr == nil
	}, time.Second, 20*time.Millisecond)
}

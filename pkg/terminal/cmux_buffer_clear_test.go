package terminal

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCmuxAdapter_SendLongText_ClearUsesIndependentContextAfterCallerCancel(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	for _, pasteFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "paste success", true: "paste failure"}[pasteFails], func(t *testing.T) {
			restoreConfig := setCmuxBufferLockConfigForTest(
				filepath.Join(t.TempDir(), "buffer.lock"), time.Second,
			)
			t.Cleanup(restoreConfig)
			original := execCommand
			originalContext := execCommandContext
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var clearContextErr error
			buildCommand := func(commandCtx context.Context, _ string, args ...string) *exec.Cmd {
				if args[0] == "paste-buffer" {
					cancel()
					if pasteFails {
						return exec.Command("false")
					}
				}
				if args[0] == "set-buffer" && args[len(args)-1] == "" {
					clearContextErr = commandCtx.Err()
				}
				return exec.Command("true")
			}
			execCommand = func(name string, args ...string) *exec.Cmd {
				return buildCommand(context.Background(), name, args...)
			}
			execCommandContext = buildCommand
			t.Cleanup(func() {
				execCommand = original
				execCommandContext = originalContext
			})

			_ = (&CmuxAdapter{}).SendLongText(ctx, "surface:7", "prompt")

			assert.NoError(t, clearContextErr)
		})
	}
}

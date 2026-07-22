package terminal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmuxAdapter_SendCommand_OptionLikePayloadUsesArgumentSeparator(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")

	for _, payload := range []string{"--help", "--workspace=workspace:999", "-h"} {
		t.Run(payload, func(t *testing.T) {
			restore, captured := newCmuxMockV2("", nil)
			defer restore()

			err := (&CmuxAdapter{}).SendCommand(context.Background(), "surface:1414", payload)

			require.NoError(t, err)
			assert.Equal(t, []string{
				"send", "--workspace", "workspace:13", "--surface", "surface:1414", "--", payload,
			}, captured.lastArgs())
		})
	}
}

func TestCmuxAdapter_SendLongText_OptionLikePayloadUsesArgumentSeparator(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:13")

	for _, payload := range []string{"--help", "--workspace=workspace:999", "-h"} {
		t.Run(payload, func(t *testing.T) {
			restore, captured := newCmuxMockV2("", nil)
			defer restore()

			err := (&CmuxAdapter{}).SendLongText(context.Background(), "surface:1414", payload)

			require.NoError(t, err)
			require.Len(t, captured.calls, 3)
			require.Len(t, captured.calls[0].args, 5)
			bufferName := captured.calls[0].args[2]
			assert.Equal(t, []string{
				"set-buffer", "--name", bufferName, "--", payload,
			}, captured.calls[0].args)
		})
	}
}

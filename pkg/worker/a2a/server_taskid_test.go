package a2a

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_EnqueueRejectsUnsafeTaskID(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{})

	err := srv.enqueueAndDispatchTask(context.Background(), nil, SendMessageParams{
		TaskID:  "index.lock file exists",
		Payload: json.RawMessage(`{}`),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "task ID")
	assert.Empty(t, srv.tasks)
}

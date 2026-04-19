package a2a

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_ReconnectTransport_ReRegistersAgentCard(t *testing.T) {
	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "reconnect-worker",
		Skills:     []string{"echo"},
		Handler: func(_ context.Context, _ string, _ json.RawMessage) (*TaskResult, error) {
			return &TaskResult{}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second) // initial registration

	mb.closeConn()
	require.NoError(t, srv.ReconnectTransport(ctx))

	msgs := mb.waitForMessages(t, 1, 3*time.Second)
	var regReq JSONRPCRequest
	require.NoError(t, json.Unmarshal(msgs[0], &regReq))
	assert.Equal(t, MethodRegisterCard, regReq.Method)
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

func TestStepSaveAndCheckProviders_UsesJWTForWorkerAndMCPSkipsBridge(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	workDir := filepath.Join(tmpHome, "work")
	require.NoError(t, os.MkdirAll(workDir, 0o755))

	prevWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workDir))
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	var workerKeyCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workspaces/ws-123/worker-keys":
			workerKeyCalls.Add(1)
			http.Error(w, "worker key path must not be used", http.StatusInternalServerError)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces/ws-123/agents":
			assert.Equal(t, "Bearer jwt-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": []map[string]any{
					{
						"id":     "memory-agent-1",
						"name":   "worker-memory",
						"type":   "dev_worker",
						"tier":   "worker",
						"status": "active",
					},
				},
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(&out)

	err = stepSaveAndCheckProviders(
		cmd,
		srv.URL,
		"jwt-token",
		&setup.Workspace{ID: "ws-123", Name: "Workspace 123"},
	)
	require.NoError(t, err)

	assert.Zero(t, workerKeyCalls.Load(), "setup must not exchange JWT for a worker API key")
	assert.Contains(t, out.String(), "JWT/refresh 토큰으로 유지")
	assert.Contains(t, out.String(), "자동 knowledge file sync는 더 이상 설정하지 않습니다")

	mcpCfg, err := setup.LoadMCPConfig(setup.DefaultMCPConfigPath())
	require.NoError(t, err)
	assert.Equal(t, "Bearer jwt-token", mcpCfg.MCPServers["autopus"].Headers["Authorization"])

	workerCfg, err := setup.LoadWorkerConfig()
	require.NoError(t, err)
	assert.Equal(t, "ws-123", workerCfg.WorkspaceID)
	assert.Equal(t, "memory-agent-1", workerCfg.MemoryAgentID)
	assert.Equal(t, 1, workerCfg.Concurrency)
}

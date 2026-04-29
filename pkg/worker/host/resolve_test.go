package host

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveProviderName_PrefersAuthenticatedConfiguredProvider(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpHome, ".codex"), 0o755))

	assert.Equal(t, "codex", ResolveProviderName([]string{"claude", "codex"}))
}

func TestResolveProviderName_FallsBackToFirstConfiguredWhenNoneAuthenticated(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	assert.Equal(t, "claude", ResolveProviderName([]string{"claude", "codex"}))
}

func TestEffectiveConcurrency(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, EffectiveConcurrency("codex", 3))
	assert.Equal(t, 1, EffectiveConcurrency("codex", 1))
	assert.Equal(t, 3, EffectiveConcurrency("claude", 3))
}

func TestResolveRuntime_BuildsResolvedConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	require.NoError(t, setup.SaveWorkerConfig(setup.WorkerConfig{
		BackendURL:  "https://api.autopus.co",
		WorkspaceID: "ws-test",
		Providers:   []string{"codex"},
		Concurrency: 3,
	}))

	customPath := filepath.Join(t.TempDir(), "desktop", "credentials.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(customPath), 0o700))
	require.NoError(t, os.WriteFile(customPath, []byte(`{"auth_type":"api_key","api_key":"acos_worker_test","backend_url":"https://api.autopus.co"}`), 0o600))

	cfg, err := ResolveRuntime(Input{CredentialsPath: customPath})
	require.NoError(t, err)

	assert.Equal(t, "codex", cfg.ProviderName)
	assert.Equal(t, "adk-worker-codex", cfg.WorkerName)
	assert.Equal(t, "acos_worker_test", cfg.AuthToken)
	assert.Equal(t, "ws-test", cfg.WorkspaceID)
	assert.Equal(t, 3, cfg.RequestedConcurrency)
	assert.Equal(t, 1, cfg.MaxConcurrency)
	assert.Equal(t, ".", cfg.WorkDir)
	assert.Equal(t, setup.DefaultMCPConfigPath(), cfg.MCPConfigPath)
	assert.Equal(t, customPath, cfg.CredentialsPath)
	assert.True(t, cfg.KnowledgeSync)
	require.NotNil(t, cfg.ProviderAdapter)
	assert.Equal(t, []string{"codex"}, cfg.LoopConfig().Providers)
}

func TestResolveRuntime_UsesCustomCredentialsPath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	require.NoError(t, setup.SaveWorkerConfig(setup.WorkerConfig{
		BackendURL:  "https://api.autopus.co",
		WorkspaceID: "ws-custom",
		Providers:   []string{"codex"},
	}))

	customPath := filepath.Join(t.TempDir(), "desktop", "credentials.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(customPath), 0o700))
	require.NoError(t, os.WriteFile(customPath, []byte(`{"auth_type":"jwt","access_token":"desktop-token","refresh_token":"desktop-refresh"}`), 0o600))

	cfg, err := ResolveRuntime(Input{CredentialsPath: customPath})
	require.NoError(t, err)

	assert.Equal(t, customPath, cfg.CredentialsPath)
	assert.Equal(t, "desktop-token", cfg.AuthToken)
	require.NotNil(t, cfg.CredentialStore)

	saved := `{"access_token":"updated-token","refresh_token":"updated-refresh"}`
	require.NoError(t, cfg.CredentialStore.Save("ignored", saved))
	data, err := os.ReadFile(customPath)
	require.NoError(t, err)
	assert.JSONEq(t, saved, string(data))
}

func TestRunSidecar_EmitsStructuredResolveFailure(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := RunSidecar(context.Background(), Input{
		ConfigPath: filepath.Join(t.TempDir(), "missing-worker.yaml"),
	}, &buf)
	require.Error(t, err)

	events := decodeEvents(t, &buf)
	require.Len(t, events, 2)
	assert.Equal(t, "runtime.starting", events[0].Event)
	assert.Equal(t, SidecarProtocolVersion, events[0].ProtocolVersion)
	assert.Equal(t, "runtime.stopped", events[1].Event)
	require.NotNil(t, events[1].Error)
	assert.Equal(t, string(ErrorConfigLoad), events[1].Error.Code)
}

func TestNDJSONEmitter_MapsHostEvents(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	emitter := NewNDJSONEmitter(&buf)
	cfg := RuntimeConfig{
		WorkerName:   "adk-worker-codex",
		WorkspaceID:  "ws-test",
		ProviderName: "codex",
	}
	observer := emitter.Observer(cfg)

	observer.OnHostEvent(worker.HostEvent{
		Type:    worker.HostEventRuntimeDegraded,
		Message: "fallback active",
	})
	observer.OnHostEvent(worker.HostEvent{
		Type:       worker.HostEventApprovalRequested,
		TaskID:     "task-1",
		ApprovalID: "approval-1",
		TraceID:    "trace-1",
		Action:     "deploy",
		RiskLevel:  "high",
		Context:    "prod",
	})
	observer.OnHostEvent(worker.HostEvent{
		Type:       worker.HostEventTaskCompleted,
		TaskID:     "task-1",
		CostUSD:    0.42,
		DurationMS: 1500,
	})

	events := decodeEvents(t, &buf)
	require.Len(t, events, 3)
	assert.Equal(t, "runtime.degraded", events[0].Event)
	assert.Equal(t, "task.approval_requested", events[1].Event)
	require.NotNil(t, events[1].Approval)
	assert.Equal(t, "approval-1", events[1].Approval.ApprovalID)
	assert.Equal(t, "trace-1", events[1].Approval.TraceID)
	assert.Equal(t, "deploy", events[1].Approval.Action)
	assert.Equal(t, "task.completed", events[2].Event)
	require.NotNil(t, events[2].Metrics)
	assert.Equal(t, int64(1500), events[2].Metrics.DurationMS)
}

func TestNDJSONEmitter_DegradesUnknownHostEvents(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	emitter := NewNDJSONEmitter(&buf)
	cfg := RuntimeConfig{
		WorkerName:   "adk-worker-codex",
		WorkspaceID:  "ws-test",
		ProviderName: "codex",
	}
	observer := emitter.Observer(cfg)

	observer.OnHostEvent(worker.HostEvent{
		Type: worker.HostEventType("mystery"),
	})

	events := decodeEvents(t, &buf)
	require.Len(t, events, 1)
	assert.Equal(t, "runtime.degraded", events[0].Event)
	require.NotNil(t, events[0].Error)
	assert.Equal(t, "unknown_host_event", events[0].Error.Code)
	assert.Contains(t, events[0].Error.Message, "mystery")
}

func decodeEvents(t *testing.T, buf *bytes.Buffer) []Event {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	events := make([]Event, 0, len(lines))
	for _, line := range lines {
		var event Event
		require.NoError(t, json.Unmarshal(line, &event))
		events = append(events, event)
	}
	return events
}

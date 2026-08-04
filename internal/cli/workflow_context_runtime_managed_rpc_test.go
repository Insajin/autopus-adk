package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextManagedRPCDriver_CleanupRejectsRootRebindAndPreservesExternalSentinel(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.Chmod(base, 0o700))
	workspace := filepath.Join(base, "workspace")
	require.NoError(t, os.Mkdir(workspace, 0o700))
	installWorkflowContextManagedLiveBridge(t, workspace)
	runtimeRoot := filepath.Join(base, "omp-runtime")
	sessionDir := filepath.Join(runtimeRoot, "sessions")
	require.NoError(t, os.MkdirAll(sessionDir, 0o700))
	configPath := filepath.Join(runtimeRoot, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("compaction: {}\n"), 0o600))
	external := t.TempDir()
	sentinel := filepath.Join(external, "sentinel")
	require.NoError(t, os.WriteFile(sentinel, []byte("preserve"), 0o600))
	driver, err := NewWorkflowContextManagedRPCDriver(WorkflowContextManagedRPCOptions{
		Executable: "/usr/bin/true", Workspace: workspace, RuntimeBase: base,
		RuntimeRoot: runtimeRoot, SessionDir: sessionDir,
		ConfigPath: configPath, Model: "fixture/model",
		AllowedEndpoint: "http://127.0.0.1:43127",
		Environment:     []string{"PI_CODING_AGENT_DIR=" + runtimeRoot, "PI_CONFIG_FILES=" + configPath},
	})
	require.NoError(t, err)

	moved := filepath.Join(base, "original-runtime")
	require.NoError(t, os.Rename(runtimeRoot, moved))
	require.NoError(t, os.Symlink(external, runtimeRoot))

	err = driver.Cleanup(context.Background())
	require.ErrorContains(t, err, "ownership changed")
	data, readErr := os.ReadFile(sentinel)
	require.NoError(t, readErr)
	assert.Equal(t, "preserve", string(data))
	assert.FileExists(t, filepath.Join(moved, workflowContextManagedRuntimeMarker))
}

func TestWorkflowContextManagedRPCDriver_DuplicateConstructorCannotShareRuntimeLease(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	require.NoError(t, os.Chmod(base, 0o700))
	workspace := filepath.Join(base, "workspace")
	require.NoError(t, os.Mkdir(workspace, 0o700))
	installWorkflowContextManagedLiveBridge(t, workspace)
	runtimeRoot := filepath.Join(base, "omp-runtime")
	sessionDir := filepath.Join(runtimeRoot, "sessions")
	require.NoError(t, os.MkdirAll(sessionDir, 0o700))
	configPath := filepath.Join(runtimeRoot, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("compaction: {}\n"), 0o600))
	options := WorkflowContextManagedRPCOptions{
		Executable: "/usr/bin/true", Workspace: workspace, RuntimeBase: base,
		RuntimeRoot: runtimeRoot, SessionDir: sessionDir,
		ConfigPath: configPath, Model: "fixture/model",
		AllowedEndpoint: "http://127.0.0.1:43127",
		Environment:     []string{"PI_CODING_AGENT_DIR=" + runtimeRoot, "PI_CONFIG_FILES=" + configPath},
	}
	first, err := NewWorkflowContextManagedRPCDriver(options)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Cleanup(context.Background()) })

	second, err := NewWorkflowContextManagedRPCDriver(options)
	require.ErrorContains(t, err, "exclusive runtime lease")
	assert.Nil(t, second)
}

func TestWorkflowContextManagedRPCProtocol_BridgeEnvelopeRejectsWrongAndReplay(t *testing.T) {
	t.Parallel()
	binding := WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion,
		BindingHash:   runtimeHash("binding"), OptionsHash: runtimeHash("options"),
		SessionHash: runtimeHash("session"), NonceHash: runtimeHash("nonce"),
	}
	protocol := &workflowContextManagedRPCProtocol{usedUI: make(map[string]struct{})}
	valid := workflowContextManagedBridgeEnvelope{
		SchemaVersion: binding.SchemaVersion, Event: WorkflowContextEventPreCompaction,
		BindingHash: binding.BindingHash, OptionsHash: binding.OptionsHash,
		SessionHash: binding.SessionHash, NonceHash: binding.NonceHash,
	}
	frame := managedRPCBridgeFrame(t, "ack-1", valid)
	event, err := protocol.bridgeRequest(frame, binding)
	require.NoError(t, err)
	assert.Equal(t, WorkflowContextEventPreCompaction, event)

	_, err = protocol.bridgeRequest(frame, binding)
	require.ErrorContains(t, err, "replayed")
	wrong := valid
	wrong.BindingHash = runtimeHash("wrong-binding")
	_, err = protocol.bridgeRequest(managedRPCBridgeFrame(t, "ack-2", wrong), binding)
	require.ErrorContains(t, err, "authority mismatch")
}

func TestWorkflowContextManagedRPCProtocol_TimeoutEOFAndAfterCloseFailBeforeDispatch(t *testing.T) {
	t.Parallel()
	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		protocol := &workflowContextManagedRPCProtocol{
			frames: make(chan []byte), done: make(chan error, 1),
		}
		require.ErrorIs(t, protocol.awaitNativeCompactionEnd(ctx), context.Canceled)
	})
	t.Run("eof", func(t *testing.T) {
		frames := make(chan []byte)
		close(frames)
		done := make(chan error, 1)
		done <- io.EOF
		protocol := &workflowContextManagedRPCProtocol{frames: frames, done: done}
		require.ErrorContains(t, protocol.awaitNativeCompactionEnd(context.Background()), "EOF")
	})
	t.Run("after close", func(t *testing.T) {
		base := t.TempDir()
		require.NoError(t, os.Chmod(base, 0o700))
		workspace := filepath.Join(base, "workspace")
		require.NoError(t, os.Mkdir(workspace, 0o700))
		installWorkflowContextManagedLiveBridge(t, workspace)
		runtimeRoot := filepath.Join(base, "omp-runtime")
		sessionDir := filepath.Join(runtimeRoot, "sessions")
		require.NoError(t, os.MkdirAll(sessionDir, 0o700))
		configPath := filepath.Join(runtimeRoot, "config.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("compaction: {}\n"), 0o600))
		driver, err := NewWorkflowContextManagedRPCDriver(WorkflowContextManagedRPCOptions{
			Executable: "/usr/bin/true", Workspace: workspace, RuntimeBase: base,
			RuntimeRoot: runtimeRoot, SessionDir: sessionDir,
			ConfigPath: configPath, Model: "fixture/model",
			AllowedEndpoint: "http://127.0.0.1:43127",
			Environment:     []string{"PI_CODING_AGENT_DIR=" + runtimeRoot, "PI_CONFIG_FILES=" + configPath},
		})
		require.NoError(t, err)
		require.NoError(t, driver.Cleanup(context.Background()))
		_, err = driver.Dispatch(context.Background(), WorkflowContextDispatch{})
		require.ErrorContains(t, err, "outside the live post hook")
		assert.False(t, driver.Observation().ProviderObserved)
	})
}

func managedRPCBridgeFrame(
	t *testing.T, id string, envelope workflowContextManagedBridgeEnvelope,
) workflowContextManagedRPCFrame {
	t.Helper()
	message, err := json.Marshal(envelope)
	require.NoError(t, err)
	raw, err := json.Marshal(string(message))
	require.NoError(t, err)
	return workflowContextManagedRPCFrame{
		ID: id, Type: "extension_ui_request", Method: "confirm",
		Title: "Autopus context " + envelope.Event, Message: raw,
	}
}

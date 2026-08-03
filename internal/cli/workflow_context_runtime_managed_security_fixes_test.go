package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextManagedRPCOptions_RejectsAmbiguousEnvironmentAndUnsafeModes(t *testing.T) {
	options, paths := newWorkflowContextManagedSecurityOptions(t)
	validEnvironment := append([]string(nil), options.Environment...)
	for _, test := range []struct {
		name        string
		environment []string
	}{
		{name: "malformed", environment: []string{"PI_CODING_AGENT_DIR", "PI_CONFIG_FILES=" + paths.config}},
		{name: "duplicate required", environment: append(validEnvironment, "PI_CODING_AGENT_DIR="+paths.runtime)},
		{name: "duplicate ambient", environment: append(validEnvironment, "PATH=/untrusted")},
		{name: "reserved bridge key", environment: append(validEnvironment, "AUTOPUS_OMP_CONTEXT_BINDING_HASH="+runtimeHash("ambient"))},
		{name: "missing config", environment: validEnvironment[:1]},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := options
			candidate.Environment = test.environment
			_, err := NewWorkflowContextManagedRPCDriver(candidate)
			require.Error(t, err)
			assert.NoFileExists(t, filepath.Join(paths.runtime, workflowContextManagedRuntimeMarker))
		})
	}

	for _, test := range []struct {
		name string
		path string
		bad  os.FileMode
		good os.FileMode
	}{
		{name: "runtime root", path: paths.runtime, bad: 0o755, good: 0o700},
		{name: "session root", path: paths.session, bad: 0o750, good: 0o700},
		{name: "config", path: paths.config, bad: 0o644, good: 0o600},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, os.Chmod(test.path, test.bad))
			_, err := NewWorkflowContextManagedRPCDriver(options)
			require.Error(t, err)
			require.NoError(t, os.Chmod(test.path, test.good))
		})
	}
}

func TestWorkflowContextInstalledManagedCanary_CleansDriverOnPreflightFailure(t *testing.T) {
	for _, test := range []struct {
		name       string
		supervisor *WorkflowContextRuntimeSupervisor
		mutate     func(*WorkflowContextRuntimeRequest)
	}{
		{name: "missing supervisor"},
		{name: "invalid identity", supervisor: NewWorkflowContextRuntimeSupervisor(nil), mutate: func(request *WorkflowContextRuntimeRequest) {
			request.Capabilities.Version = "unobserved"
		}},
		{name: "loopback not authorized", supervisor: NewWorkflowContextRuntimeSupervisor(nil), mutate: func(request *WorkflowContextRuntimeRequest) {
			request.Capabilities.AuthNoneLoopback = false
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			options, paths := newWorkflowContextManagedSecurityOptions(t)
			driver, err := NewWorkflowContextManagedRPCDriver(options)
			require.NoError(t, err)
			request := newWorkflowContextRuntimeFixture(t)
			if test.mutate != nil {
				test.mutate(&request)
			}

			_, err = RunWorkflowContextInstalledManagedCanary(
				context.Background(), test.supervisor, request, driver,
			)

			require.Error(t, err)
			assert.NoDirExists(t, paths.runtime)
			artifacts, countErr := driver.ArtifactCount(context.Background())
			require.NoError(t, countErr)
			assert.Zero(t, artifacts)
		})
	}
}

func TestWorkflowContextManagedRPCDriver_RejectsUnleasedExtraAndSourceDrift(t *testing.T) {
	t.Run("unleased extra root artifact", func(t *testing.T) {
		options, paths := newWorkflowContextManagedSecurityOptions(t)
		sentinel := filepath.Join(paths.runtime, "external-sentinel")
		require.NoError(t, os.WriteFile(sentinel, []byte("preserve"), 0o600))
		driver, err := NewWorkflowContextManagedRPCDriver(options)
		require.ErrorContains(t, err, "unowned artifact")
		assert.Nil(t, driver)
		assert.FileExists(t, sentinel)
		assert.NoFileExists(t, filepath.Join(paths.runtime, workflowContextManagedRuntimeMarker))
	})

	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, paths workflowContextManagedSecurityPaths)
	}{
		{name: "config hash", mutate: func(t *testing.T, paths workflowContextManagedSecurityPaths) {
			require.NoError(t, os.WriteFile(paths.config, []byte("compaction: []\n"), 0o600))
		}},
		{name: "bridge hash", mutate: func(t *testing.T, paths workflowContextManagedSecurityPaths) {
			body, err := os.ReadFile(paths.bridge)
			require.NoError(t, err)
			body[len(body)/2] ^= 1
			require.NoError(t, os.WriteFile(paths.bridge, body, 0o644))
		}},
		{name: "ambient extension", mutate: func(t *testing.T, paths workflowContextManagedSecurityPaths) {
			require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(paths.bridge), "ambient.ts"), []byte("export {}"), 0o600))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			options, paths := newWorkflowContextManagedSecurityOptions(t)
			driver, err := NewWorkflowContextManagedRPCDriver(options)
			require.NoError(t, err)
			t.Cleanup(func() { _ = driver.Cleanup(context.Background()) })
			test.mutate(t, paths)
			require.Error(t, driver.verifyManagedSourceIdentities())
		})
	}
}

func TestWorkflowContextManagedRPCFrames_CancellationUnblocksFullChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	frames, done := workflowContextManagedRPCFrames(ctx, reader)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for index := 0; index < 512; index++ {
			_, _ = io.WriteString(writer, "{\"type\":\"frame\"}\n")
		}
	}()
	deadline := time.Now().Add(time.Second)
	for len(frames) < cap(frames) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	require.Equal(t, cap(frames), len(frames))
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("frame reader did not stop after cancellation")
	}
	_ = reader.Close()
	_ = writer.Close()
	<-writerDone
}

func TestWorkflowContextRuntimeManaged_SourceChangeBlocksCleanedDriverFallback(t *testing.T) {
	request := newWorkflowContextRuntimeFixture(t)
	driver := &recordingManagedWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
	}, artifacts: 1}
	request.Driver = driver
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	acceptance := filepath.Join(request.Binding.DeliveryOptions.Root, filepath.FromSlash(runtimeSpecDir), "acceptance.md")
	driver.before = func(event WorkflowContextRuntimeEvent) {
		if event.Kind == WorkflowContextEventPostCompaction {
			require.NoError(t, os.WriteFile(acceptance, []byte("changed canonical acceptance"), 0o600))
		}
	}
	request.CanonicalSource = workflowContextCanonicalSourceFunc(func(
		_ context.Context, opts promptlayer.ContextDeliveryOptions,
	) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
		delivery, err := promptlayer.BuildContextDelivery(opts)
		return delivery, request.Binding.Ephemeral, err
	})

	receipt, err := NewWorkflowContextRuntimeSupervisor(nil).RunManaged(context.Background(), request)

	require.Error(t, err)
	assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
	assert.Equal(t, "canonical-full-managed-driver-reuse-blocked", receipt.Fallback.Reason)
	assert.Equal(t, config.OMPContextHistoryShadow, receipt.Mode.EffectiveHistoryMode)
	assert.Zero(t, driver.dispatchCalls)
	assert.Equal(t, 1, driver.cleanupCalls)
	assert.NotContains(t, receipt.PhaseSequence, "admitted")
}

type workflowContextManagedSecurityPaths struct {
	runtime string
	session string
	config  string
	bridge  string
}

func newWorkflowContextManagedSecurityOptions(
	t *testing.T,
) (WorkflowContextManagedRPCOptions, workflowContextManagedSecurityPaths) {
	t.Helper()
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	require.NoError(t, os.Mkdir(workspace, 0o700))
	installWorkflowContextManagedLiveBridge(t, workspace)
	runtimeRoot := filepath.Join(base, "omp-runtime")
	session := filepath.Join(runtimeRoot, "sessions")
	require.NoError(t, os.MkdirAll(session, 0o700))
	configPath := filepath.Join(runtimeRoot, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("compaction: {}\n"), 0o600))
	bridge := filepath.Join(workspace, ".omp", "extensions", "autopus-context.ts")
	paths := workflowContextManagedSecurityPaths{runtime: runtimeRoot, session: session, config: configPath, bridge: bridge}
	return WorkflowContextManagedRPCOptions{
		Executable: "/usr/bin/true", Workspace: workspace, RuntimeBase: base,
		RuntimeRoot: runtimeRoot, SessionDir: session, ConfigPath: configPath,
		Model: "fixture/model", AllowedEndpoint: "http://127.0.0.1:43127",
		Environment: []string{
			"PI_CODING_AGENT_DIR=" + runtimeRoot,
			"PI_CONFIG_FILES=" + configPath,
			"PATH=" + os.Getenv("PATH"),
		},
	}, paths
}

func assertNoManagedMarker(t *testing.T, runtime string) {
	t.Helper()
	_, err := os.Lstat(filepath.Join(runtime, workflowContextManagedRuntimeMarker))
	assert.True(t, errors.Is(err, os.ErrNotExist), strings.TrimSpace(err.Error()))
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const productDocumentCanary = "canonical-product-document-body-must-not-serialize"

func TestWorkflowContextProductSession_AssemblesAuthorityAndRunsManagedHermetically(t *testing.T) {
	input, runtime, driver, factory := newWorkflowContextProductFixture(t)
	expectedDelivery, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: input.ProjectDir, Command: input.Command, SpecDir: input.SpecDir,
	})
	require.NoError(t, err)
	expectedBinding, err := promptlayer.BuildOMPContextBinding(promptlayer.OMPContextBindingInput{
		WorkspaceID: "autopus-adk", SpecID: input.SpecID, TaskID: input.TaskID,
		Phase: input.Phase, SessionID: input.SessionID,
		DeliveryOptions: promptlayer.ContextDeliveryOptions{
			Root: input.ProjectDir, Command: input.Command, SpecDir: input.SpecDir,
		},
		Delivery: expectedDelivery,
		Ephemeral: promptlayer.OMPContextEphemeral{
			OriginalTask: input.OriginalTask, DecisionDelta: input.DecisionDelta,
			FrozenFindingIDs: input.FrozenFindingIDs, OwnershipPaths: input.OwnershipPaths,
			ForbiddenPaths: input.ForbiddenPaths,
		},
		History: runtime.History,
	})
	require.NoError(t, err)

	receipt, err := RunWorkflowContextProductSession(context.Background(), input, runtime)
	require.NoError(t, err)
	assert.Equal(t, WorkflowContextOutcomeAdmitted, receipt.Outcome)
	assert.Equal(t, []string{"checkpointed", "compacted", "rehydrated", "admitted"}, receipt.PhaseSequence)
	assert.Equal(t, expectedBinding.BindingHash, driver.binding.BindingHash)
	assert.Equal(t, expectedBinding.OptionsHash, driver.binding.OptionsHash)
	assert.Equal(t, expectedBinding.FullDocumentRefs, receipt.FullDocumentRefs)
	assert.Equal(t, WorkflowContextLifecycleReceipt{
		RequiredCompactionCycles: 1, PreCompactionACKs: 1, PostCompactionACKs: 1,
		NativeStarts: 1, NativeEnds: 1, CanonicalReadmissions: 1, EphemeralReadmissions: 1,
		ProviderTurns:           3,
		ProviderAuthorityDigest: workflowContextRuntimeHash("recording-loopback-authority"),
		SameProcess:             true, SameSession: true, ProviderObserved: true,
	}, receipt.Lifecycle)
	assert.Equal(t, expectedDelivery.Prompt, driver.dispatchedPrompt)
	assert.Equal(t, input.OriginalTask, driver.dispatchedOriginalTask)
	assert.Equal(t, 1, factory.calls)
	verified := factory.driver.(*workflowContextProductIdentityDriverSpy)
	assert.Equal(t, 1, verified.verifyCalls)
	assert.Equal(t, runtime.Capabilities.Version, verified.verifyVersion)
	assert.Equal(t, input.ProjectDir, factory.options.ProjectDir)
	assert.Equal(t, []string{input.OriginalTask, input.DecisionDelta}, factory.options.Prompts)
	assert.Equal(t, runtime.DriverOptions.Workspace, factory.options.Workspace)
	assert.NotEqual(t, input.ProjectDir, factory.options.Workspace,
		"the bridge workspace must remain task-owned and separate from the project CWD")
	assert.Equal(t, 1, driver.bindCalls)
	assert.Equal(t, 1, driver.runCalls)
	assert.Equal(t, 1, driver.dispatchCalls)
	assert.Equal(t, 1, driver.cleanupCalls)
	assert.True(t, receipt.Cleanup.Verified)
	assert.Zero(t, receipt.ArtifactCounts.AfterCleanup)
	assert.Empty(t, receipt.DocumentOmissions)
	assert.Empty(t, receipt.MemoryInjections)
	assert.FileExists(t, filepath.Join(input.ProjectDir, filepath.FromSlash(
		WorkflowContextReceiptRelativePath(input.TaskID, input.SessionID),
	)))

	serialized, err := json.Marshal(receipt)
	require.NoError(t, err)
	for _, forbidden := range []string{
		input.OriginalTask, input.DecisionDelta, productDocumentCanary,
		input.ProjectDir, runtime.DriverOptions.Workspace, runtime.DriverOptions.RuntimeBase, runtime.DriverOptions.RuntimeRoot,
		runtime.DriverOptions.SessionDir, runtime.DriverOptions.ConfigPath,
	} {
		assert.NotContains(t, string(serialized), forbidden)
	}
	assertProductOMPConfigsUnchanged(t, input.ProjectDir)
	assert.Equal(t, "/missing/hermetic-omp", factory.options.Executable,
		"the default test must use the injected managed seam, not exec or an external provider")
}
func TestWorkflowContextProductSession_RejectsIncompleteCorrelatedMultiCompactionEvidence(t *testing.T) {
	input, runtime, driver, _ := newWorkflowContextProductFixture(t)
	runtime.DriverOptions.CompactionCycles = 2
	driver.events = append(driver.events, driver.events...)
	driver.observation = &WorkflowContextManagedRPCObservation{
		ProviderTurns: 4, PreACKs: 2, PostACKs: 1, NativeStarts: 2, NativeEnds: 2,
		CanonicalReadmissions: 2, EphemeralReadmissions: 2,
		SameProcess: true, SameSession: true, Sandboxed: true, ProviderObserved: true,
		ProviderAuthorityDigest: workflowContextRuntimeHash("recording-loopback-authority"),
	}

	receipt, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

	require.ErrorContains(t, err, "multi-compaction lifecycle evidence is incomplete")
	assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
	assert.Equal(t, "managed-lifecycle-evidence-incomplete", receipt.Fallback.Reason)
	assert.Equal(t, 2, receipt.Lifecycle.RequiredCompactionCycles)
	assert.Equal(t, 1, receipt.Lifecycle.PostCompactionACKs)
	assert.True(t, receipt.Cleanup.Verified)
}

func TestWorkflowContextProductSession_SourceMutationFailsClosedOnChangedBinding(t *testing.T) {
	input, runtime, driver, _ := newWorkflowContextProductFixture(t)
	acceptancePath := filepath.Join(input.ProjectDir, filepath.FromSlash(input.SpecDir), "acceptance.md")
	driver.before = func(event WorkflowContextRuntimeEvent) {
		if event.Kind == WorkflowContextEventPostCompaction {
			require.NoError(t, os.WriteFile(acceptancePath, []byte("mutated authority"), 0o600))
		}
	}

	receipt, err := RunWorkflowContextProductSession(context.Background(), input, runtime)
	require.Error(t, err)
	assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
	assert.Equal(t, "canonical-full-managed-driver-reuse-blocked", receipt.Fallback.Reason)
	assert.False(t, receipt.ExactMatch)
	assert.Zero(t, driver.dispatchCalls)
	assert.Equal(t, 1, driver.cleanupCalls)
	assert.True(t, receipt.Cleanup.Verified)
	assertProductOMPConfigsUnchanged(t, input.ProjectDir)
}

func TestWorkflowContextProductSession_PromptAuthorityMustMatchCommandAndSpec(t *testing.T) {
	tests := []struct {
		name, prompt string
	}{
		{name: "command mismatch", prompt: "/auto fix SPEC-OMP-004 --auto"},
		{name: "spec mismatch", prompt: "/auto go SPEC-OMP-999 --auto"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, runtime, _, factory := newWorkflowContextProductFixture(t)
			input.OriginalTask = test.prompt
			_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)
			require.Error(t, err)
			assert.Zero(t, factory.calls)
		})
	}
}

func TestWorkflowContextProductSession_ProductionCobraTreeDelegatesIntentOnly(t *testing.T) {
	intent := workflowContextProductIntentV1{
		SchemaVersion: workflowContextProductIntentSchemaVersion,
		OriginalTask:  "/auto go SPEC-OMP-004 --auto",
		DecisionDelta: "continue exactly",
	}
	payload, err := json.Marshal(intent)
	require.NoError(t, err)
	var observed workflowContextProductIntentV1
	calls := 0
	root := NewRootCmd()
	root.SetContext(withWorkflowContextProductSessionEntrypoint(root.Context(), func(
		_ context.Context, got workflowContextProductIntentV1,
	) (WorkflowContextRuntimeReceipt, error) {
		calls++
		observed = got
		return WorkflowContextRuntimeReceipt{
			SchemaVersion: WorkflowContextRuntimeReceiptSchemaVersion,
			Event:         "terminal", TaskID: "trusted-task", SessionID: "trusted-session",
			Outcome: WorkflowContextOutcomeAdmitted,
		}, nil
	}))
	var stdout bytes.Buffer
	root.SetIn(bytes.NewReader(payload))
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	root.SetArgs([]string{"workflow", "context-runtime", "session", "--request-json", "-", "--format", "json"})

	require.NoError(t, root.Execute(), stdout.String())
	assert.Equal(t, 1, calls)
	assert.Equal(t, intent, observed)
	assert.NotContains(t, stdout.String(), intent.OriginalTask)
	assert.NotContains(t, stdout.String(), intent.DecisionDelta)
	command, args, err := root.Find([]string{"workflow", "context-runtime", "session"})
	require.NoError(t, err)
	assert.Empty(t, args)
	assert.Equal(t, "auto workflow context-runtime session", command.CommandPath())
}

type workflowContextProductDriverFactorySpy struct {
	driver  WorkflowContextManagedProcessDriver
	calls   int
	options WorkflowContextManagedRPCOptions
}

func (factory *workflowContextProductDriverFactorySpy) New(
	options WorkflowContextManagedRPCOptions,
) (WorkflowContextManagedProcessDriver, error) {
	factory.calls++
	factory.options = options
	return factory.driver, nil
}

func newWorkflowContextProductFixture(t *testing.T) (
	WorkflowContextProductSessionInput,
	WorkflowContextProductRuntimeInputs,
	*recordingManagedWorkflowContextDriver,
	*workflowContextProductDriverFactorySpy,
) {
	t.Helper()
	base := newWorkflowContextRuntimeFixture(t)
	base.Binding.Ephemeral = promptlayer.OMPContextEphemeral{
		OriginalTask:     "/auto go SPEC-OMP-004 --auto",
		DecisionDelta:    "continue with the authoritative task",
		FrozenFindingIDs: []string{"F-002", "F-010"}, OwnershipPaths: []string{"internal/cli"},
		ForbiddenPaths: []string{"pkg/promptlayer", "pkg/adapter/omp"},
	}
	require.NoError(t, os.WriteFile(filepath.Join(base.Binding.DeliveryOptions.Root, "AGENTS.md"), []byte(productDocumentCanary), 0o600))
	delivery, err := promptlayer.BuildContextDelivery(base.Binding.DeliveryOptions)
	require.NoError(t, err)
	base.Binding.Delivery = delivery
	attachValidWorkflowContextPromotion(t, &base)

	cfg := config.DefaultFullConfig("autopus-adk")
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{
		Profile: "active",
		Profiles: map[string]config.OMPContextProfileConf{"active": {
			HistoryMode: base.Policy.HistoryMode, MemoryMode: base.Policy.MemoryMode,
			HistoryTargetTokens: base.Policy.HistoryTargetTokens, Fallback: base.Policy.Fallback,
			CapabilityPolicy: base.Policy.CapabilityPolicy, RuntimeRootPolicy: base.Policy.RuntimeRootPolicy,
			MutationScope: base.Policy.MutationScope,
		}},
	}
	require.NoError(t, config.Save(base.Binding.DeliveryOptions.Root, cfg))

	driver := &recordingManagedWorkflowContextDriver{events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
	}, artifacts: 1}
	verifiedDriver := &workflowContextProductIdentityDriverSpy{recordingManagedWorkflowContextDriver: driver}
	factory := &workflowContextProductDriverFactorySpy{driver: verifiedDriver}
	input := WorkflowContextProductSessionInput{
		ProjectDir: base.Binding.DeliveryOptions.Root,
		Command:    "go", SpecDir: runtimeSpecDir,
		SpecID: base.Binding.SpecID, TaskID: base.Binding.TaskID, Phase: base.Binding.Phase,
		SessionID: base.Binding.SessionID, OriginalTask: base.Binding.Ephemeral.OriginalTask,
		DecisionDelta:    base.Binding.Ephemeral.DecisionDelta,
		FrozenFindingIDs: base.Binding.Ephemeral.FrozenFindingIDs,
		OwnershipPaths:   base.Binding.Ephemeral.OwnershipPaths, ForbiddenPaths: base.Binding.Ephemeral.ForbiddenPaths,
	}
	runtimeBase := t.TempDir()
	bridgeWorkspace := filepath.Join(runtimeBase, "bridge-workspace")
	runtimeRoot := filepath.Join(runtimeBase, "runtime")
	sessionDir := filepath.Join(runtimeRoot, "sessions")
	require.NoError(t, os.MkdirAll(bridgeWorkspace, 0o700))
	require.NoError(t, os.MkdirAll(sessionDir, 0o700))
	runtime := WorkflowContextProductRuntimeInputs{
		Capabilities: base.Capabilities, Promotion: base.Promotion, History: base.Binding.History,
		Overlay: baseOverlay(t), Supervisor: NewWorkflowContextRuntimeSupervisor(nil),
		ReceiptWriter: &WorkflowContextReceiptWriter{WorkspaceRoot: input.ProjectDir},
		DriverOptions: WorkflowContextManagedRPCOptions{
			Executable: "/missing/hermetic-omp", ProjectDir: input.ProjectDir, Workspace: bridgeWorkspace,
			RuntimeBase: runtimeBase, RuntimeRoot: runtimeRoot, SessionDir: sessionDir,
			ConfigPath: filepath.Join(runtimeRoot, "config.json"),
			Model:      "hermetic/model", AllowedEndpoint: "http://127.0.0.1:1",
			HistoryAfterTokens: map[string]int{"old-read": 2},
		},
		NewManagedDriver: factory.New,
	}
	return input, runtime, driver, factory
}

func baseOverlay(t *testing.T) WorkflowContextOverlayController {
	t.Helper()
	return newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
}

func assertProductOMPConfigsUnchanged(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"settings.json", "config.json", "config.yaml"} {
		assert.NoFileExists(t, filepath.Join(root, ".omp", name))
	}
	path := filepath.Join(root, ".omp", "config.yml")
	if info, err := os.Lstat(path); err == nil {
		assert.True(t, info.Mode().IsRegular())
		assert.Zero(t, info.Mode()&os.ModeSymlink)
	} else {
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextRuntimeManaged_InvalidDispatchACKBlocksAdmissionAndRollsBack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		mutateACK     func(*WorkflowContextDispatchAck)
		zeroACK       bool
		dispatchError bool
		duplicatePost bool
	}{
		{name: "missing_zero_ack", zeroACK: true},
		{name: "wrong_schema", mutateACK: func(ack *WorkflowContextDispatchAck) { ack.SchemaVersion = "autopus.omp-context-dispatch-ack.v0" }},
		{name: "wrong_binding_hash", mutateACK: func(ack *WorkflowContextDispatchAck) { ack.BindingHash = sameLengthHashMismatch(ack.BindingHash) }},
		{name: "wrong_options_hash", mutateACK: func(ack *WorkflowContextDispatchAck) { ack.OptionsHash = sameLengthHashMismatch(ack.OptionsHash) }},
		{name: "wrong_session_hash", mutateACK: func(ack *WorkflowContextDispatchAck) { ack.SessionHash = sameLengthHashMismatch(ack.SessionHash) }},
		{name: "wrong_nonce_hash", mutateACK: func(ack *WorkflowContextDispatchAck) { ack.NonceHash = sameLengthHashMismatch(ack.NonceHash) }},
		{name: "provider_not_observed", mutateACK: func(ack *WorkflowContextDispatchAck) { ack.ProviderObserved = false }},
		{name: "dispatch_error", dispatchError: true},
		{name: "duplicate_post_dispatch_attempt", duplicatePost: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := newWorkflowContextRuntimeFixture(t)
			events := []WorkflowContextRuntimeEvent{
				{Kind: WorkflowContextEventPreCompaction},
				{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
				{Kind: WorkflowContextEventPostCompaction},
			}
			if tt.duplicatePost {
				events = append(events, WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPostCompaction})
			}
			driver := &recordingManagedWorkflowContextDriver{events: events, artifacts: 1}
			if tt.dispatchError {
				driver.dispatchErr = workflowContextDispatchError()
			}
			driver.ackFactory = func(binding WorkflowContextBridgeBinding, _ WorkflowContextDispatch) WorkflowContextDispatchAck {
				if tt.zeroACK {
					return WorkflowContextDispatchAck{}
				}
				ack := validWorkflowContextDispatchAck(binding)
				if tt.mutateACK != nil {
					tt.mutateACK(&ack)
				}
				return ack
			}
			request.Driver = driver
			overlay := newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
			request.Overlay = overlay
			rebuildCalls := 0
			request.CanonicalSource = workflowContextCanonicalSourceFunc(func(_ context.Context, opts promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
				rebuildCalls++
				delivery, err := promptlayer.BuildContextDelivery(opts)
				return delivery, request.Binding.Ephemeral, err
			})
			store := promptlayer.NewOMPContextTransientStore()

			receipt, err := NewWorkflowContextRuntimeSupervisor(store).RunManaged(context.Background(), request)

			require.Error(t, err)
			assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
			assert.NotContains(t, receipt.PhaseSequence, "admitted")
			assert.Equal(t, 1, rebuildCalls)
			assert.Equal(t, 1, driver.dispatchCalls)
			assert.Equal(t, 1, driver.cleanupCalls)
			assert.True(t, receipt.Cleanup.Verified)
			assert.Equal(t, config.OMPContextHistoryShadow, receipt.Mode.EffectiveHistoryMode)
			assert.Empty(t, overlay.readbacks, "failure must consume the rollback readback")
			assert.Zero(t, store.Pending(), "terminal failure must release and zeroize transient state")
		})
	}
}

func TestWorkflowContextRuntimeManaged_BindingAndDispatchACKJSONAreBodyFree(t *testing.T) {
	t.Parallel()
	binding := WorkflowContextBridgeBinding{
		SchemaVersion: "autopus.omp-context-bridge.v1",
		BindingHash:   runtimeHash("binding"), OptionsHash: runtimeHash("options"),
		SessionHash: runtimeHash("session"), NonceHash: runtimeHash("nonce"),
	}
	ack := validWorkflowContextDispatchAck(binding)
	tests := []struct {
		name     string
		value    any
		wantKeys []string
	}{
		{name: "binding", value: binding, wantKeys: []string{"schema_version", "binding_hash", "options_hash", "session_hash", "nonce_hash"}},
		{name: "ack", value: ack, wantKeys: []string{"schema_version", "binding_hash", "options_hash", "session_hash", "nonce_hash", "provider_observed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.value)
			require.NoError(t, err)
			var object map[string]any
			require.NoError(t, json.Unmarshal(data, &object))
			keys := make([]string, 0, len(object))
			for key := range object {
				keys = append(keys, key)
			}
			assert.ElementsMatch(t, tt.wantKeys, keys)
			serialized := string(data)
			assert.NotContains(t, serialized, "prompt")
			assert.NotContains(t, serialized, "body")
			assert.NotContains(t, serialized, "transient")
			assert.NotContains(t, serialized, "/Users/")
			assert.NotContains(t, serialized, "implement runtime supervisor")
		})
	}
}

func sameLengthHashMismatch(value string) string {
	if strings.HasSuffix(value, "0") {
		return strings.TrimSuffix(value, "0") + "1"
	}
	return value[:len(value)-1] + "0"
}

func assertRuntimeSHA256(t *testing.T, value string) {
	t.Helper()
	assert.True(t, strings.HasPrefix(value, "sha256:"))
	assert.Len(t, strings.TrimPrefix(value, "sha256:"), 64)
}

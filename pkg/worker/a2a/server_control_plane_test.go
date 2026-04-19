package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeTaskPayload_InjectsModelAndPipelineMetadata(t *testing.T) {
	payload, err := mergeTaskPayload(
		json.RawMessage(`{"prompt":"hello"}`),
		"gpt-5.4",
		[]string{"planner", "reviewer"},
		map[string]string{"planner": "Plan carefully."},
		map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"},
		&IterationBudget{Limit: 15, WarnThreshold: 0.7, DangerThreshold: 0.9},
	)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, "gpt-5.4", decoded["model"])
	assert.Equal(t, []interface{}{"planner", "reviewer"}, decoded["pipeline_phases"])
	assert.Equal(t, map[string]any{"planner": "Plan carefully."}, decoded["pipeline_instructions"])
	assert.Equal(t, map[string]any{"planner": "SERVER TEMPLATE\n\n{{input}}"}, decoded["pipeline_prompt_templates"])
	assert.Equal(t, map[string]any{"limit": float64(15), "warn_threshold": 0.7, "danger_threshold": 0.9}, decoded["iteration_budget"])
}

func TestServer_SendMessage_MissingPolicySignatureRejectedWhenSecretConfigured(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "test-secret")

	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
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

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second)

	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:         "task-no-sig",
			Payload:        json.RawMessage(`{"prompt":"test"}`),
			SecurityPolicy: SecurityPolicy{TimeoutSec: 60},
		}),
	}
	data, err := json.Marshal(taskReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	msgs := mb.waitForMessages(t, 1, 5*time.Second)
	var finalResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[0], &finalResp))
	require.NotNil(t, finalResp.Error)
	assert.Contains(t, finalResp.Error.Message, "missing policy signature")
}

func TestServer_SendMessage_MissingControlPlaneSignatureRejectedWhenSecretConfigured(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "test-secret")

	mb := newMockBackend()
	defer mb.close()

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
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

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second)

	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:                   "task-no-control-plane-sig",
			Payload:                  json.RawMessage(`{"prompt":"test"}`),
			Model:                    "gpt-5.4",
			ControlPlaneCapabilities: []string{CapabilityServerModelV1},
			PolicySignature:          mustSignPolicy(t, "task-no-control-plane-sig", SecurityPolicy{TimeoutSec: 60}, "test-secret"),
			SecurityPolicy:           SecurityPolicy{TimeoutSec: 60},
		}),
	}
	data, err := json.Marshal(taskReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	msgs := mb.waitForMessages(t, 1, 5*time.Second)
	var finalResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(msgs[0], &finalResp))
	require.NotNil(t, finalResp.Error)
	assert.Contains(t, finalResp.Error.Message, "missing control plane signature")
}

func TestServer_SendMessage_ControlPlaneCapabilitiesFilterMetadata(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "test-secret")

	mb := newMockBackend()
	defer mb.close()

	modelSeen := make(chan string, 1)
	handler := func(_ context.Context, _ string, payload json.RawMessage) (*TaskResult, error) {
		var msg struct {
			Model                   string            `json:"model"`
			PipelinePhases          []string          `json:"pipeline_phases"`
			PipelineInstructions    map[string]string `json:"pipeline_instructions"`
			PipelinePromptTemplates map[string]string `json:"pipeline_prompt_templates"`
			IterationBudget         *IterationBudget  `json:"iteration_budget"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		modelSeen <- msg.Model
		if len(msg.PipelinePhases) != 0 || len(msg.PipelineInstructions) != 0 || len(msg.PipelinePromptTemplates) != 0 || msg.IterationBudget != nil {
			return nil, fmt.Errorf("unexpected unauthorized control plane metadata")
		}
		return &TaskResult{}, nil
	}

	srv := NewServer(ServerConfig{
		BackendURL: mb.wsURL(),
		WorkerName: "test-worker",
		Skills:     []string{"echo"},
		Handler:    handler,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		require.NoError(t, srv.Close())
	}()

	srv.config.BackendURL = mb.wsURL()
	require.NoError(t, srv.Start(ctx))
	mb.waitForMessages(t, 1, 3*time.Second)

	policy := SecurityPolicy{TimeoutSec: 60}
	controlPlaneCaps := []string{CapabilityServerModelV1}
	taskReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  MethodSendMessage,
		Params: mustMarshal(SendMessageParams{
			TaskID:                   "task-filter-control-plane",
			Payload:                  json.RawMessage(`{"prompt":"test"}`),
			Model:                    "gpt-5.4",
			PipelinePhases:           []string{"planner", "reviewer"},
			PipelineInstructions:     map[string]string{"planner": "Plan carefully."},
			PipelinePromptTemplates:  map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"},
			IterationBudget:          &IterationBudget{Limit: 12, WarnThreshold: 0.7, DangerThreshold: 0.9},
			ControlPlaneCapabilities: controlPlaneCaps,
			ControlPlaneSignature:    mustSignControlPlane(t, "task-filter-control-plane", "gpt-5.4", []string{"planner", "reviewer"}, map[string]string{"planner": "Plan carefully."}, map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"}, &IterationBudget{Limit: 12, WarnThreshold: 0.7, DangerThreshold: 0.9}, controlPlaneCaps, "test-secret"),
			PolicySignature:          mustSignPolicy(t, "task-filter-control-plane", policy, "test-secret"),
			SecurityPolicy:           policy,
		}),
	}
	data, err := json.Marshal(taskReq)
	require.NoError(t, err)
	require.NoError(t, mb.sendMessage(data))

	select {
	case got := <-modelSeen:
		assert.Equal(t, "gpt-5.4", got)
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not receive filtered control plane metadata")
	}
}

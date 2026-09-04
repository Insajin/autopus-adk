package omp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type modelRoleRPCFakeRunner struct {
	roles    map[string]string
	rpcCalls atomic.Int64
}

func (runner *modelRoleRPCFakeRunner) Run(
	context.Context,
	string,
	...string,
) ([]byte, error) {
	return nil, fmt.Errorf("config get must not read modelRoles when RPC is available")
}

func (runner *modelRoleRPCFakeRunner) RunWithInput(
	_ context.Context,
	executable string,
	input []byte,
	args ...string,
) ([]byte, error) {
	if executable != cliBinary || string(input) != `{"id":"autopus-model-state","type":"get_state"}`+"\n" {
		return nil, fmt.Errorf("unexpected RPC request")
	}
	role := ""
	for index, arg := range args {
		if arg == "--model" && index+1 < len(args) {
			role = strings.TrimPrefix(args[index+1], "@")
		}
	}
	selector, ok := runner.roles[role]
	if !ok {
		return nil, fmt.Errorf("unknown role %q", role)
	}
	modelSelector, thinking, err := splitOMPProjectedSelector(selector)
	if err != nil {
		return nil, err
	}
	provider, model, ok := parseOMPRoutingSelector(modelSelector)
	if !ok {
		return nil, fmt.Errorf("invalid selector")
	}
	runner.rpcCalls.Add(1)
	frame := map[string]any{
		"id": "autopus-model-state", "type": "response", "command": "get_state", "success": true,
		"data": map[string]any{
			"model":         map[string]any{"provider": provider, "id": model},
			"thinkingLevel": thinking,
		},
	}
	encoded, err := json.Marshal(frame)
	return append(encoded, '\n'), err
}

func TestReadOMPModelExpectedValues_UsesProviderFreeRPCForEveryRole(t *testing.T) {
	t.Parallel()
	roles := map[string]string{
		"autopus_reviewer": "anthropic/claude-opus-5:max",
		"autopus_planner":  "openai-codex/gpt-5.6-sol:max",
		"autopus_executor": "openai-codex/gpt-5.6-terra:high",
	}
	runner := &modelRoleRPCFakeRunner{roles: roles}

	readback, err := ReadOMPModelExpectedValues(
		context.Background(), runner, "/tmp/config.yml", map[string]any{"modelRoles": roles},
	)

	require.NoError(t, err)
	require.Equal(t, int64(len(roles)), runner.rpcCalls.Load())
	var decoded map[string]map[string]string
	require.NoError(t, json.Unmarshal(readback, &decoded))
	require.Equal(t, roles, decoded["modelRoles"])
}

package omp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
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

// The role readback spawns one omp process per role, so the argv shape is a
// trust boundary: only the provider-free rpc session form may reach the pin.
func TestSafeOMPModelRoleRPCArgs_AcceptsOnlyProviderFreeRoleSessions(t *testing.T) {
	t.Parallel()
	good := []string{"--config", "/tmp/routing.yml", "--model", "@autopus_executor", "--mode", "rpc",
		"--no-tools", "--no-skills", "--no-extensions"}
	require.True(t, SafeOMPModelRoleRPCArgs(good))

	mutate := func(index int, value string) []string {
		bad := append([]string(nil), good...)
		bad[index] = value
		return bad
	}
	for name, args := range map[string][]string{
		"relative config":   mutate(1, "routing.yml"),
		"newline in config": mutate(1, "/tmp/a\nb.yml"),
		"selector model":    mutate(3, "anthropic/claude-opus-5"),
		"unsafe role":       mutate(3, "@role;rm"),
		"non-rpc mode":      mutate(5, "cli"),
		"tools enabled":     mutate(6, "--tools"),
		"missing flag":      good[:8],
		"extra flag":        append(append([]string(nil), good...), "--yolo"),
	} {
		require.False(t, SafeOMPModelRoleRPCArgs(args), name)
	}
}

// concurrencyProbeRunner records how many role sessions overlap so the
// worker bound is observable, and can corrupt one role's answer.
type concurrencyProbeRunner struct {
	modelRoleRPCFakeRunner
	inFlight atomic.Int64
	peak     atomic.Int64
	corrupt  string
}

func (runner *concurrencyProbeRunner) RunWithInput(
	ctx context.Context, executable string, input []byte, args ...string,
) ([]byte, error) {
	current := runner.inFlight.Add(1)
	defer runner.inFlight.Add(-1)
	for {
		peak := runner.peak.Load()
		if current <= peak || runner.peak.CompareAndSwap(peak, current) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	output, err := runner.modelRoleRPCFakeRunner.RunWithInput(ctx, executable, input, args...)
	if err == nil && runner.corrupt != "" && strings.Contains(strings.Join(args, " "), "@"+runner.corrupt+" ") {
		output = []byte(strings.Replace(string(output), `"thinkingLevel":"xhigh"`, `"thinkingLevel":"low"`, 1))
	}
	return output, err
}

func sixteenAgentRoles() map[string]string {
	roles := make(map[string]string, len(config.CanonicalAgentNames()))
	for _, agent := range config.CanonicalAgentNames() {
		roles[config.OMPAgentRoleName(agent)] = "anthropic/claude-opus-5:xhigh"
	}
	return roles
}

func TestReadOMPModelRolesViaRPC_BoundsInFlightSessionsToWorkerCount(t *testing.T) {
	t.Parallel()
	roles := sixteenAgentRoles()
	runner := &concurrencyProbeRunner{modelRoleRPCFakeRunner: modelRoleRPCFakeRunner{roles: roles}}

	resolved, err := readOMPModelRolesViaRPC(context.Background(), runner, "/tmp/config.yml", roles)

	require.NoError(t, err)
	require.Equal(t, roles, resolved)
	require.Equal(t, int64(len(roles)), runner.rpcCalls.Load())
	require.LessOrEqual(t, runner.peak.Load(), int64(ompModelRoleRPCWorkers))
	require.Greater(t, runner.peak.Load(), int64(1), "readback must actually overlap sessions")
}

func TestReadOMPModelRolesViaRPC_FailsWholeReadbackOnOneMismatchedRole(t *testing.T) {
	t.Parallel()
	roles := sixteenAgentRoles()
	runner := &concurrencyProbeRunner{
		modelRoleRPCFakeRunner: modelRoleRPCFakeRunner{roles: roles}, corrupt: "autopus_tester",
	}

	resolved, err := readOMPModelRolesViaRPC(context.Background(), runner, "/tmp/config.yml", roles)

	require.Nil(t, resolved)
	require.ErrorContains(t, err, "activation role readback mismatch: autopus_tester")
}

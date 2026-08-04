package omp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPReadiness_DefaultRunnerCompletesCredentialFreeBehavioralProbe(t *testing.T) {
	if !supportsOMPReadinessBehaviorProcessGroup() {
		t.Skip("behavioral readiness fails closed without POSIX process groups")
	}
	report := ProbeOMPReadiness(context.Background(), behavioralFixtureOptions(t, "healthy"))

	require.Len(t, report.Capabilities, 10)
	for _, capability := range report.Capabilities {
		assert.True(t, capability.Supported, "%s reason=%s", capability.ID, capability.Reason)
	}
	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	for _, forbidden := range []string{"Authorization", "readiness-receipt.json", t.TempDir()} {
		assert.NotContains(t, string(encoded), forbidden)
	}
}

func TestOMPReadiness_BehavioralProbeFailsClosed(t *testing.T) {
	if !supportsOMPReadinessBehaviorProcessGroup() {
		t.Skip("behavioral readiness fails closed without POSIX process groups")
	}
	tests := []struct {
		mode, reason string
	}{
		{mode: "missing", reason: "event_missing"},
		{mode: "malformed", reason: "output_invalid"},
		{mode: "nonzero", reason: "exit_nonzero"},
		{mode: "timeout", reason: "timeout"},
	}
	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			report := ProbeOMPReadiness(ctx, behavioralFixtureOptions(t, tc.mode))
			if tc.mode == "missing" {
				assert.True(t, capabilityByID(t, report, "rpc.command_discovery").Supported)
				assert.True(t, capabilityByID(t, report, "rpc.tool_events").Supported)
			} else {
				for _, id := range []string{"rpc.command_discovery", "rpc.tool_events", "rpc.terminal"} {
					assert.False(t, capabilityByID(t, report, id).Supported, id)
				}
			}
			assert.False(t, capabilityByID(t, report, "rpc.terminal").Supported)
			assert.Equal(t, tc.reason, capabilityByID(t, report, "rpc.terminal").Reason)
		})
	}
}

func TestOMPReadinessProvider_RejectsUnsafeRequestsAndBudgetOverflow(t *testing.T) {
	tests := []struct {
		name, path, model, tool, auth, want string
		authPresent, oversized              bool
	}{
		{name: "authorization", path: ompReadinessCompletionPath, model: ompReadinessModel, tool: "write", auth: "Bearer secret", authPresent: true, want: "authorization_present"},
		{name: "empty authorization", path: ompReadinessCompletionPath, model: ompReadinessModel, tool: "write", authPresent: true, want: "authorization_present"},
		{name: "endpoint", path: "/v1/responses", model: ompReadinessModel, tool: "write", want: "unexpected_endpoint"},
		{name: "model", path: ompReadinessCompletionPath, model: "wrong-model", tool: "write", want: "invalid_request"},
		{name: "tool", path: ompReadinessCompletionPath, model: ompReadinessModel, tool: "read", want: "invalid_request"},
		{name: "oversized", path: ompReadinessCompletionPath, model: ompReadinessModel, tool: "write", oversized: true, want: "request_oversized"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scratch := t.TempDir()
			require.NoError(t, os.Chmod(scratch, 0o700))
			provider, err := startOMPReadinessBehaviorProvider(scratch)
			require.NoError(t, err)
			defer provider.Close()
			body := behavioralProviderRequest(tc.model, tc.tool, "first")
			if tc.oversized {
				body = bytes.Repeat([]byte("x"), ompReadinessRequestMaxBytes+1)
			}
			request, err := http.NewRequest(http.MethodPost, provider.URL()+tc.path, bytes.NewReader(body))
			require.NoError(t, err)
			if tc.authPresent {
				request.Header["Authorization"] = []string{tc.auth}
			}
			response, err := http.DefaultClient.Do(request)
			require.NoError(t, err)
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			assert.Equal(t, http.StatusBadRequest, response.StatusCode)
			assert.Equal(t, tc.want, provider.FailureReason())
		})
	}

	t.Run("request budget", func(t *testing.T) {
		scratch := t.TempDir()
		require.NoError(t, os.Chmod(scratch, 0o700))
		provider, err := startOMPReadinessBehaviorProvider(scratch)
		require.NoError(t, err)
		defer provider.Close()
		for index, stage := range []string{"first", "second", "third"} {
			response, postErr := http.Post(provider.URL()+ompReadinessCompletionPath,
				"application/json", bytes.NewReader(behavioralProviderRequest(ompReadinessModel, "write", stage)))
			require.NoError(t, postErr)
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if index < ompReadinessRequestBudget {
				assert.Equal(t, http.StatusOK, response.StatusCode)
			} else {
				assert.Equal(t, http.StatusBadRequest, response.StatusCode)
			}
		}
		assert.Equal(t, "request_budget_exceeded", provider.FailureReason())
	})
}

func TestOMPReadinessBehavior_UsesTaskProfileAndHardensNewReceipt(t *testing.T) {
	overlay, err := createOMPReadinessOverlay()
	require.NoError(t, err)
	base := filepath.Dir(overlay)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(base)) })
	runner, _ := configureOMPProbeRunner(commandOMPProbeRunner{maxOutput: 1024}, os.Args[0], overlay)
	environment := environmentMapOMP(runner.environment)
	profile := filepath.Join(base, "pi-agent")
	assert.Equal(t, profile, environment["PI_CODING_AGENT_DIR"])
	require.NoError(t, writeOMPReadinessModelConfig(profile, "http://127.0.0.1:1"))
	modelInfo, err := os.Stat(filepath.Join(profile, "models.yml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), modelInfo.Mode().Perm())

	receipt := filepath.Join(base, "scratch-receipt")
	require.NoError(t, os.WriteFile(receipt, []byte(ompReadinessReceiptContent), 0o644))
	assert.True(t, secureOMPReadinessReceipt(receipt))
	receiptInfo, err := os.Stat(receipt)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), receiptInfo.Mode().Perm())
}

func behavioralFixtureOptions(t *testing.T, mode string, fixtureArgs ...string) OMPReadinessOptions {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	prefixArgs := []string{"-test.run=^TestOMPReadinessBehaviorFixtureProcess$", "--", "--fixture-mode=" + mode}
	prefixArgs = append(prefixArgs, fixtureArgs...)
	return OMPReadinessOptions{
		Root: t.TempDir(), Executable: executable, Timeout: 3 * time.Second, MaxOutput: 64 * 1024,
		Runner: commandOMPProbeRunner{maxOutput: 64 * 1024, prefixArgs: prefixArgs},
	}
}

func TestOMPReadinessBehaviorFixtureProcess(t *testing.T) {
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		return
	}
	os.Exit(runOMPReadinessBehaviorFixture(os.Args[separator+1:]))
}

func runOMPReadinessBehaviorFixture(args []string) int {
	mode := strings.TrimPrefix(args[0], "--fixture-mode=")
	args = args[1:]
	pidPath := ""
	if len(args) > 0 && strings.HasPrefix(args[0], "--fixture-pid=") {
		pidPath = strings.TrimPrefix(args[0], "--fixture-pid=")
		args = args[1:]
	}
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "--version"):
		_, _ = os.Stdout.WriteString("omp/17.1.8\n")
	case strings.Contains(joined, "--help"):
		_, _ = os.Stdout.WriteString("--mode <interactive|rpc> --no-session --cwd <path> --model <provider/model>\n")
	case strings.Contains(joined, "config get"):
		_, _ = os.Stdout.WriteString("[\".agents/skills\"]\n")
	case strings.Contains(joined, "models --json"):
		_, _ = os.Stdout.WriteString("{\"models\":[]}\n")
	case strings.Contains(joined, "--mode rpc"):
		return runOMPReadinessBehaviorRPCFixture(mode, pidPath, args)
	default:
		return 71
	}
	return 0
}

func runOMPReadinessBehaviorRPCFixture(mode, pidPath string, args []string) int {
	if mode == "timeout" {
		time.Sleep(10 * time.Second)
		return 72
	}
	if mode == "timeout-child" {
		child := exec.Command("/bin/sleep", "30")
		if child.Start() != nil || os.WriteFile(pidPath, []byte(strconv.Itoa(child.Process.Pid)), 0o600) != nil {
			return 78
		}
		time.Sleep(10 * time.Second)
		return 79
	}
	if mode == "nonzero" {
		return 73
	}
	if !validBehavioralStdin(os.Stdin) {
		return 74
	}
	baseURL := readinessFixtureBaseURL()
	if baseURL == "" || postBehavioralFixture(baseURL, "first") != nil {
		return 75
	}
	root := valueAfterBehaviorArg(args, "--cwd")
	if os.WriteFile(filepath.Join(root, ompReadinessReceiptName), []byte(ompReadinessReceiptContent), 0o600) != nil {
		return 76
	}
	if postBehavioralFixture(baseURL, "second") != nil {
		return 77
	}
	_, _ = os.Stdout.WriteString("{\"type\":\"available_commands_update\"}\n")
	_, _ = os.Stdout.WriteString("{\"type\":\"tool_execution_start\",\"toolCallId\":\"ready-1\"}\n")
	_, _ = os.Stdout.WriteString("{\"type\":\"tool_execution_end\",\"toolCallId\":\"ready-1\"}\n")
	if mode == "malformed" {
		_, _ = os.Stdout.WriteString("not-json\n{\"type\":\"message_end\"}\n")
	} else if mode != "missing" {
		_, _ = os.Stdout.WriteString("{\"type\":\"message_end\"}\n")
	}
	return 0
}

func validBehavioralStdin(reader io.Reader) bool {
	scanner := bufio.NewScanner(reader)
	want := []string{"set_auto_retry", "set_auto_compaction", "prompt"}
	for index := 0; index < len(want); index++ {
		if !scanner.Scan() {
			return false
		}
		var frame map[string]any
		if json.Unmarshal(scanner.Bytes(), &frame) != nil || frame["type"] != want[index] {
			return false
		}
	}
	return scanner.Err() == nil
}

func readinessFixtureBaseURL() string {
	data, err := os.ReadFile(filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "models.yml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if value := strings.TrimSpace(strings.TrimPrefix(trimmed, "baseUrl:")); strings.HasPrefix(trimmed, "baseUrl:") && value != "" {
			return value
		}
	}
	return ""
}

func postBehavioralFixture(baseURL, stage string) error {
	response, err := http.Post(baseURL+"/chat/completions", "application/json",
		bytes.NewReader(behavioralProviderRequest(ompReadinessModel, "write", stage)))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, ompReadinessProviderResponseMaxBytes+1))
	if response.StatusCode != http.StatusOK {
		return assert.AnError
	}
	return nil
}

func behavioralProviderRequest(model, tool, stage string) []byte {
	request := map[string]any{
		"model": model, "stream": true,
		"tools":    []any{map[string]any{"type": "function", "function": map[string]any{"name": tool}}},
		"messages": []any{map[string]any{"role": "user", "content": stage + " " + ompReadinessReceiptName}},
	}
	encoded, _ := json.Marshal(request)
	return encoded
}

func valueAfterBehaviorArg(args []string, key string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key {
			return args[index+1]
		}
	}
	return ""
}

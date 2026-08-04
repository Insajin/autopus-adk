package omp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPReadiness_LiveInstalledBinaryReportsAllCapabilities(t *testing.T) {
	if os.Getenv("AUTOPUS_OMP_READINESS_LIVE") != "1" {
		t.Skip("set AUTOPUS_OMP_READINESS_LIVE=1 to probe the installed OMP binary")
	}
	if _, err := exec.LookPath("omp"); err != nil {
		t.Skip("installed OMP binary is unavailable")
	}

	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	report := ProbeOMPReadiness(ctx, OMPReadinessOptions{Root: root, Executable: "omp"})

	wantIDs := []string{
		"identity.version",
		"launch.rpc",
		"launch.no_session",
		"launch.cwd",
		"launch.model",
		"config.overlay_readback",
		"catalog.models_json",
		"rpc.command_discovery",
		"rpc.tool_events",
		"rpc.terminal",
	}
	require.Len(t, report.Capabilities, len(wantIDs))
	for index, capability := range report.Capabilities {
		t.Logf("capability=%s supported=%t reason=%s", capability.ID, capability.Supported, capability.Reason)
		assert.Equal(t, wantIDs[index], capability.ID)
		assert.True(t, capability.Supported, "%s reason=%s", capability.ID, capability.Reason)
	}
	assert.Regexp(t, regexp.MustCompile(`^omp/[0-9]+\.[0-9]+\.[0-9]+$`), report.Version)

	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	for _, forbidden := range []string{
		root,
		ompReadinessReceiptName,
		ompReadinessReceiptContent,
		"Authorization",
		"apiKey",
		"models.yml",
	} {
		assert.NotContains(t, string(encoded), forbidden)
	}
}

func TestOMPReadiness_LiveInstalledBinaryBehaviorDiagnostics(t *testing.T) {
	if os.Getenv("AUTOPUS_OMP_READINESS_LIVE") != "1" {
		t.Skip("set AUTOPUS_OMP_READINESS_LIVE=1 to probe the installed OMP binary")
	}
	if _, err := exec.LookPath("omp"); err != nil {
		t.Skip("installed OMP binary is unavailable")
	}

	overlay, err := createOMPReadinessOverlay()
	require.NoError(t, err)
	base := filepath.Dir(overlay)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(base)) })
	scratch := filepath.Join(base, "scratch")
	require.NoError(t, os.Mkdir(scratch, 0o700))
	provider, err := startOMPReadinessBehaviorProvider(scratch)
	require.NoError(t, err)
	defer provider.Close()
	require.NoError(t, writeOMPReadinessModelConfig(filepath.Join(base, "pi-agent"), provider.URL()))

	runner, _ := configureOMPProbeRunner(commandOMPProbeRunner{maxOutput: 64 * 1024}, "omp", overlay)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	started := time.Now()
	output, runErr := runner.runRPC(ctx, "omp", scratch, ompReadinessRPCInput(),
		"--config", overlay,
		"--mode", "rpc",
		"--no-session",
		"--cwd", scratch,
		"--model", ompReadinessProviderID+"/"+ompReadinessModel,
		"--tools", "write",
		"--auto-approve",
		"--no-extensions",
		"--no-rules",
		"--no-lsp",
		"--no-pty",
	)

	provider.mu.Lock()
	requests, stages, authHeaders := provider.requests, provider.stages, provider.authHeaders
	providerFailure := provider.failure
	provider.mu.Unlock()
	runReason := "ok"
	if runErr != nil {
		runReason = classifyOMPProbeError(ctx, runErr)
	}
	discovery, pairedTools, terminal, valid := parseOMPRPCEvents(output)
	events, responses := boundedOMPRPCDiagnostics(output)
	receiptValid := secureOMPReadinessReceipt(filepath.Join(scratch, ompReadinessReceiptName))
	t.Logf("run=%s wall_ms=%d provider=%s requests=%d stages=%d auth=%d rpc_valid=%t discovery=%t tools=%t terminal=%t receipt=%t events=%s responses=%s",
		runReason, time.Since(started).Milliseconds(), boundedOMPReadinessDiagnostic(providerFailure), requests, stages, authHeaders,
		valid, discovery, pairedTools, terminal, receiptValid, events, responses)

	assert.NoError(t, runErr)
	assert.Empty(t, providerFailure)
	assert.Equal(t, ompReadinessRequestBudget, requests)
	assert.Equal(t, ompReadinessRequestBudget, stages)
	assert.Zero(t, authHeaders)
	assert.True(t, valid)
	assert.True(t, discovery)
	assert.True(t, pairedTools)
	assert.True(t, terminal)
	assert.True(t, receiptValid)
}

func TestOMPReadiness_LiveInstalledBinaryStaticProbeTimings(t *testing.T) {
	if os.Getenv("AUTOPUS_OMP_READINESS_LIVE") != "1" {
		t.Skip("set AUTOPUS_OMP_READINESS_LIVE=1 to probe the installed OMP binary")
	}
	if _, err := exec.LookPath("omp"); err != nil {
		t.Skip("installed OMP binary is unavailable")
	}

	overlay, err := createOMPReadinessOverlay()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(filepath.Dir(overlay))) })
	runner, _ := configureOMPProbeRunner(commandOMPProbeRunner{maxOutput: 64 * 1024}, "omp", overlay)
	opts := OMPReadinessOptions{
		Root: t.TempDir(), Executable: "omp", Runner: runner,
		Timeout: 3 * time.Second, MaxOutput: 64 * 1024,
	}
	probes := []struct {
		name string
		args []string
	}{
		{name: "version", args: []string{"--version"}},
		{name: "help", args: []string{"--help"}},
		{name: "config", args: []string{"--config", overlay, "config", "get", "skills.customDirectories", "--json"}},
		{name: "catalog", args: []string{"--config", overlay, "models", "--json"}},
	}
	for _, probe := range probes {
		started := time.Now()
		result := runOMPProbe(context.Background(), opts, probe.args...)
		elapsed := time.Since(started)
		t.Logf("probe=%s wall_ms=%d reason=%s", probe.name, elapsed.Milliseconds(), boundedOMPReadinessDiagnostic(result.reason))
		assert.Empty(t, result.reason, probe.name)
		assert.Less(t, elapsed, opts.Timeout, probe.name)
	}
}

func boundedOMPRPCDiagnostics(output []byte) (string, string) {
	events := make([]string, 0)
	responses := make([]string, 0)
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var frame struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Success bool   `json:"success"`
			Error   string `json:"error"`
			Message struct {
				Role string `json:"role"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &frame) != nil {
			events = append(events, "invalid")
			continue
		}
		event := frame.Type
		if (frame.Type == "message_start" || frame.Type == "message_end") && frame.Message.Role != "" {
			event += ":" + frame.Message.Role
		}
		events = append(events, event)
		if frame.Type == "response" {
			responses = append(responses, frame.Command+":"+boundedOMPResponseStatus(frame.Success, frame.Error))
		}
	}
	return strings.Join(events, ","), strings.Join(responses, ",")
}

func boundedOMPResponseStatus(success bool, message string) string {
	if success {
		return "ok"
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "model"), strings.Contains(lower, "provider"):
		return "model_unavailable"
	case strings.Contains(lower, "credential"), strings.Contains(lower, "api key"), strings.Contains(lower, "auth"):
		return "credential_unavailable"
	case strings.Contains(lower, "unknown"):
		return "command_unknown"
	default:
		return "failed"
	}
}

func boundedOMPReadinessDiagnostic(reason string) string {
	switch reason {
	case "", "request_budget_exceeded", "unexpected_endpoint", "authorization_present",
		"request_oversized", "invalid_request", "response_invalid", "timeout",
		"exit_nonzero", "output_oversized", "output_invalid":
		if reason == "" {
			return "none"
		}
		return reason
	default:
		return "redacted"
	}
}

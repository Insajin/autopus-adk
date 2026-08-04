package omp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type ompPartialOutputRunner struct {
	output []byte
	err    error
}

func (runner ompPartialOutputRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return append([]byte(nil), runner.output...), runner.err
}

func TestOMPReadinessOverlay_ConfigReadbackUsesPIConfigFiles(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	binDir := t.TempDir()
	overlayDir := t.TempDir()
	overlay := filepath.Join(overlayDir, "config.yml")
	executable := filepath.Join(binDir, "omp-overlay-fixture")
	script := `#!/bin/sh
printf 'pi_config=%s\n' "$PI_CONFIG_FILES"
printf 'args=%s\n' "$*"
`
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("PI_CONFIG_FILES", "/user/config.yml")

	runner := commandOMPProbeRunner{maxOutput: 4096}
	output, err := runner.Run(context.Background(), filepath.Base(executable),
		"--config", overlay, "config", "get", "skills.customDirectories", "--json")
	if err != nil {
		t.Fatal(err)
	}
	want := "pi_config=" + overlay + "\nargs=config get skills.customDirectories --json\n"
	if string(output) != want {
		t.Fatalf("config readback invocation mismatch\nwant: %q\n got: %q", want, output)
	}
}

func TestOMPReadinessOverlay_ActualInstalledConfigReadback(t *testing.T) {
	executable, err := exec.LookPath(cliBinary)
	if err != nil {
		t.Skip("installed omp is unavailable")
	}
	overlay, err := createOMPReadinessOverlay()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cleanupErr := os.RemoveAll(filepath.Dir(overlay)); cleanupErr != nil {
			t.Errorf("remove readiness overlay: %v", cleanupErr)
		}
	})

	assertOMPReadinessOverlayModes(t, overlay)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := (commandOMPProbeRunner{maxOutput: 4096}).Run(ctx, executable,
		"--config", overlay, "config", "get", "skills.customDirectories", "--json")
	if err != nil {
		t.Fatalf("installed config readback failed: %v; output=%q", err, output)
	}
	var response struct {
		Key         string   `json:"key"`
		Value       []string `json:"value"`
		Type        string   `json:"type"`
		Description string   `json:"description"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		t.Fatalf("installed config readback returned invalid JSON: %v; output=%q", err, output)
	}
	if response.Key != "skills.customDirectories" || response.Type != "array" ||
		len(response.Value) != 1 || response.Value[0] != ".agents/skills" {
		t.Fatalf("installed config readback mismatch: %+v", response)
	}
	capability := evaluateOMPConfigCapability(ompProbeResult{output: output})
	if !capability.Supported || capability.Reason != "output_valid" {
		t.Fatalf("installed config readback was not accepted: %+v", capability)
	}
	if strings.Contains(string(output), overlay) {
		t.Fatal("config readback output retained the task-owned overlay path")
	}
}

func assertOMPReadinessOverlayModes(t *testing.T, overlay string) {
	t.Helper()
	dirInfo, err := os.Stat(filepath.Dir(overlay))
	if err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(overlay)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 || fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("overlay modes must be 0700/0600, got %04o/%04o",
			dirInfo.Mode().Perm(), fileInfo.Mode().Perm())
	}
}

func TestOMPReadinessOverlay_ConfigWrapperIsStrict(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		supported bool
	}{
		{
			name: "installed exact wrapper",
			payload: `{"key":"skills.customDirectories","value":[".agents/skills"],` +
				`"type":"array","description":""}`,
			supported: true,
		},
		{
			name: "unexpected secret field",
			payload: `{"key":"skills.customDirectories","value":[".agents/skills"],` +
				`"type":"array","description":"","secret":"forbidden"}`,
		},
		{
			name: "wrong key",
			payload: `{"key":"skills.other","value":[".agents/skills"],` +
				`"type":"array","description":""}`,
		},
		{
			name: "wrong type",
			payload: `{"key":"skills.customDirectories","value":[".agents/skills"],` +
				`"type":"string","description":""}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			capability := evaluateOMPConfigCapability(ompProbeResult{output: []byte(tc.payload)})
			if capability.Supported != tc.supported {
				t.Fatalf("supported=%v, want %v; result=%+v", capability.Supported, tc.supported, capability)
			}
		})
	}
}

func TestOMPReadinessOverlay_PartialRPCEventPreservesTerminationReason(t *testing.T) {
	tests := []struct {
		name   string
		reason string
	}{
		{name: "timeout", reason: "timeout"},
		{name: "nonzero", reason: "exit_nonzero"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ompProbeResult{
				output: []byte("{\"type\":\"available_commands_update\"}\n"),
				reason: tc.reason,
			}
			capabilities := evaluateOMPRPCCapabilities(result)
			if capabilities[0].Supported ||
				capabilities[0].Reason != "event_observed_partial_"+tc.reason {
				t.Fatalf("partial evidence must remain diagnostic-only: %+v", capabilities[0])
			}
			for _, capability := range capabilities[1:] {
				if capability.Supported || capability.Reason != tc.reason {
					t.Fatalf("unobserved capability must retain termination reason: %+v", capability)
				}
			}
		})
	}
}

func TestOMPReadinessOverlay_RunProbeRetainsBoundedPartialOutput(t *testing.T) {
	partial := []byte("{\"type\":\"available_commands_update\"}\n")
	opts := OMPReadinessOptions{
		Executable: "omp",
		Runner: ompPartialOutputRunner{
			output: partial,
			err:    context.DeadlineExceeded,
		},
		Timeout:   time.Second,
		MaxOutput: 1024,
	}
	result := runOMPProbe(context.Background(), opts, "--mode", "rpc")
	if result.reason != "timeout" {
		t.Fatalf("unexpected timeout classification: %+v", result)
	}
	if string(result.output) != string(partial) {
		t.Fatalf("partial output was discarded: %q", result.output)
	}
}

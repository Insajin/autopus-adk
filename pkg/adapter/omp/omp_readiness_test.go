package omp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const readinessCatalog = `{"models":[{"provider":"s7dummy","id":"s7-probe","available":false}]}`

type readinessFakeRunner struct {
	outputs       map[string][]byte
	errors        map[string]error
	calls         [][]string
	modelRequests int
}

func newReadinessFakeRunner(root string) *readinessFakeRunner {
	rawRPC := strings.Join([]string{
		`{"type":"available_commands_update","commands":["auto","auto-plan"]}`,
		`{"type":"tool_execution_start","toolCallId":"call-1","toolName":"write"}`,
		`{"type":"tool_execution_end","toolCallId":"call-1","isError":false}`,
		`{"type":"message_update","credential":"sk-readiness-secret","path":` + string(mustJSON(root)) + `}`,
		`{"type":"message_end"}`,
	}, "\n")
	return &readinessFakeRunner{
		outputs: map[string][]byte{
			"version": []byte("omp/17.1.8\n"),
			"help":    []byte("--mode <interactive|rpc> --no-session --cwd <path> --model <provider/model> --config <path>"),
			"config":  []byte(`[".agents/skills"]`),
			"models":  []byte(readinessCatalog),
			"rpc":     []byte(rawRPC),
		},
		errors: make(map[string]error),
	}
}

func mustJSON(value string) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func (f *readinessFakeRunner) Run(_ context.Context, executable string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{executable}, args...))
	key := readinessCallKey(args)
	if key == "model_request" {
		f.modelRequests++
	}
	if err := f.errors[key]; err != nil {
		return nil, err
	}
	return append([]byte(nil), f.outputs[key]...), nil
}

func readinessCallKey(args []string) string {
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "--version"):
		return "version"
	case strings.Contains(joined, "--help"):
		return "help"
	case strings.Contains(joined, "config get"):
		return "config"
	case strings.Contains(joined, "models --json"):
		return "models"
	case strings.Contains(joined, "--mode rpc"):
		return "rpc"
	default:
		return "model_request"
	}
}

func readinessOptions(t *testing.T, runner OMPProbeRunner) OMPReadinessOptions {
	t.Helper()
	return OMPReadinessOptions{
		Root:       t.TempDir(),
		Executable: "omp",
		Runner:     runner,
		Timeout:    100 * time.Millisecond,
		MaxOutput:  4 * 1024,
		Selectors:  []string{"s7dummy/s7-probe", "s7dummy/", "unknown-family"},
	}
}

func capabilityByID(t *testing.T, report OMPReadinessReport, id string) OMPCapabilityResult {
	t.Helper()
	for _, result := range report.Capabilities {
		if result.ID == id {
			return result
		}
	}
	t.Fatalf("missing capability result %q", id)
	return OMPCapabilityResult{}
}

func selectorByValue(t *testing.T, report OMPReadinessReport, selector string) OMPSelectorResolution {
	t.Helper()
	for _, result := range report.SelectorResolutions {
		if result.Selector == selector {
			return result
		}
	}
	t.Fatalf("missing selector result %q", selector)
	return OMPSelectorResolution{}
}

func TestOMPReadiness_ReportsExactCapabilitiesAndSelectorResolution(t *testing.T) {
	root := t.TempDir()
	runner := newReadinessFakeRunner(root)
	opts := OMPReadinessOptions{
		Root: root, Executable: "omp", Runner: runner,
		Timeout: 100 * time.Millisecond, MaxOutput: 4 * 1024,
		Selectors: []string{"s7dummy/s7-probe", "s7dummy/", "unknown-family"},
	}

	report := ProbeOMPReadiness(context.Background(), opts)
	assert.Equal(t, "omp/17.1.8", report.Version)

	wantReasons := map[string]string{
		"identity.version":        "version_verified",
		"launch.rpc":              "flag_present",
		"launch.no_session":       "flag_present",
		"launch.cwd":              "flag_present",
		"launch.model":            "flag_present",
		"config.overlay_readback": "output_valid",
		"catalog.models_json":     "output_valid",
		"rpc.command_discovery":   "event_observed",
		"rpc.tool_events":         "event_observed",
		"rpc.terminal":            "event_observed",
	}
	require.Len(t, report.Capabilities, len(wantReasons))
	for id, reason := range wantReasons {
		result := capabilityByID(t, report, id)
		assert.True(t, result.Supported, id)
		assert.Equal(t, reason, result.Reason, id)
	}

	assert.Equal(t, OMPSelectorResolution{
		Selector: "s7dummy/s7-probe", ResolvedModel: "s7dummy/s7-probe",
		Status: "resolved", Reason: "credential_unavailable",
	}, selectorByValue(t, report, "s7dummy/s7-probe"))
	assert.Equal(t, "selector_malformed", selectorByValue(t, report, "s7dummy/").Reason)
	assert.Equal(t, "selector_unresolved", selectorByValue(t, report, "unknown-family").Reason)
	assert.Zero(t, runner.modelRequests, "catalog discovery must not issue a completion or model request")

	receipt, err := json.Marshal(report)
	require.NoError(t, err)
	for _, forbidden := range []string{root, "sk-readiness-secret", "available_commands_update", readinessCatalog} {
		assert.NotContains(t, string(receipt), forbidden)
	}
}

func TestOMPReadiness_ClassifiesCapabilityFailures(t *testing.T) {
	tests := []struct {
		name, capability, reason string
		mutate                   func(*readinessFakeRunner)
	}{
		{"missing flag", "launch.cwd", "flag_missing", func(f *readinessFakeRunner) {
			f.outputs["help"] = []byte("--mode rpc --no-session --model value --config value")
		}},
		{"invalid output", "config.overlay_readback", "output_invalid", func(f *readinessFakeRunner) {
			f.outputs["config"] = []byte(`{"skills":"wrong"}`)
		}},
		{"missing event", "rpc.terminal", "event_missing", func(f *readinessFakeRunner) {
			f.outputs["rpc"] = []byte(`{"type":"available_commands_update"}`)
		}},
		{"timeout", "catalog.models_json", "timeout", func(f *readinessFakeRunner) {
			f.errors["models"] = context.DeadlineExceeded
		}},
		{"nonzero", "rpc.terminal", "exit_nonzero", func(f *readinessFakeRunner) {
			f.errors["rpc"] = errors.New("exit status 7")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := newReadinessFakeRunner(t.TempDir())
			tc.mutate(runner)
			report := ProbeOMPReadiness(context.Background(), readinessOptions(t, runner))
			assert.Equal(t, "omp/17.1.8", report.Version)
			result := capabilityByID(t, report, tc.capability)
			assert.False(t, result.Supported)
			assert.Equal(t, tc.reason, result.Reason)
		})
	}
}

func TestOMPReadiness_CatalogShapeAndFailureReasons(t *testing.T) {
	tests := []struct {
		name, payload, catalogReason, capabilityReason string
		err                                            error
		maxOutput                                      int
		capabilitySupported                            bool
	}{
		{"empty", `{"models":[]}`, "catalog_empty", "output_valid", nil, 4096, true},
		{"wrong top level", `[]`, "catalog_invalid", "output_invalid", nil, 4096, false},
		{"extra top level", `{"models":[],"raw":"forbidden"}`, "catalog_invalid", "output_invalid", nil, 4096, false},
		{"malformed", `{"models":[`, "catalog_invalid", "output_invalid", nil, 4096, false},
		{"oversized", strings.Repeat("x", 33), "catalog_oversized", "output_invalid", nil, 32, false},
		{"timeout", "", "catalog_timeout", "timeout", context.DeadlineExceeded, 4096, false},
		{"nonzero", "", "catalog_exit_nonzero", "exit_nonzero", errors.New("exit status 9"), 4096, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := newReadinessFakeRunner(t.TempDir())
			runner.outputs["models"] = []byte(tc.payload)
			runner.errors["models"] = tc.err
			opts := readinessOptions(t, runner)
			opts.MaxOutput = tc.maxOutput
			report := ProbeOMPReadiness(context.Background(), opts)
			assert.Equal(t, tc.catalogReason, report.CatalogReason)
			capability := capabilityByID(t, report, "catalog.models_json")
			assert.Equal(t, tc.capabilitySupported, capability.Supported)
			assert.Equal(t, tc.capabilityReason, capability.Reason)
			assert.Zero(t, runner.modelRequests)
		})
	}
}

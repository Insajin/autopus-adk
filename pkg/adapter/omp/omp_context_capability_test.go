package omp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ompContextCapabilityFakeRunner struct {
	outputs map[string][]byte
	errors  map[string]error
}

func (runner ompContextCapabilityFakeRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	return runner.outputs[key], runner.errors[key]
}

func TestProbeOMPContextCapabilities_BoundsFailuresAndModes(t *testing.T) {
	t.Parallel()
	runner := ompContextCapabilityFakeRunner{outputs: map[string][]byte{
		"--version": []byte(strings.Repeat("x", 65)),
	}, errors: map[string]error{"config get compaction --json": context.DeadlineExceeded, "--help": errors.New("failed")}}
	report := ProbeOMPContextCapabilities(context.Background(), OMPContextCapabilityOptions{Runner: runner, MaxOutput: 64, RequestedHistoryMode: "off", RequestedMemoryMode: "invalid"})
	require.Empty(t, report.Version)
	require.Equal(t, "off", report.EffectiveHistoryMode)
	require.Equal(t, "off", report.EffectiveMemoryMode)
	require.Equal(t, "output_oversized", ompContextCapabilityByID(t, report, "identity.version").Reason)
	require.Equal(t, "timeout", ompContextCapabilityByID(t, report, "config.compaction_schema").Reason)
	require.Equal(t, "exit_nonzero", ompContextCapabilityByID(t, report, "persistence.no_session").Reason)

	missing := ProbeOMPContextCapabilities(context.Background(), OMPContextCapabilityOptions{})
	require.Equal(t, "runner_missing", ompContextCapabilityByID(t, missing, "identity.version").Reason)
	require.Equal(t, OMPContextLifecycleEvidence{}, ParseOMPContextLifecycleEvidence([]byte("invalid\n{}")))
}

func TestProbeOMPContextCapabilities_RequiresLifecycleAndSupervisorAdmissionEvidence(t *testing.T) {
	t.Parallel()
	complete := ompContextCapabilityFakeRunner{outputs: map[string][]byte{
		"--version":                    []byte("omp/17.1.8"),
		"config get compaction --json": []byte(`{"enabled":true}`),
		"config get memory --json":     []byte(`{"enabled":false}`),
		"--help":                       []byte("--no-session --mode rpc"),
		"--mode rpc --no-session":      []byte("{\"type\":\"auto_compaction_start\"}\n{\"type\":\"auto_compaction_end\"}\n{\"type\":\"autopus_context_reinjection_ready\"}\n{\"type\":\"autopus_context_admission_blocked\"}\n{\"type\":\"autopus_context_cleanup_verified\"}\n{\"type\":\"autopus_context_memory_intercepted\"}\n"),
	}}
	report := ProbeOMPContextCapabilities(context.Background(), OMPContextCapabilityOptions{
		Runner: complete, Timeout: time.Second, MaxOutput: 4096, RequestedHistoryMode: "active", RequestedMemoryMode: "off",
	})
	require.Equal(t, "omp/17.1.8", report.Version)
	require.Equal(t, "active", report.EffectiveHistoryMode)
	require.Equal(t, "off", report.EffectiveMemoryMode)
	require.True(t, report.ActiveEligible)
	require.Len(t, report.Capabilities, 11)
	for _, capability := range report.Capabilities {
		require.True(t, capability.Supported, capability.ID+":"+capability.Reason)
	}

	versionOnly := complete
	versionOnly.outputs = cloneOMPContextOutputs(complete.outputs)
	versionOnly.outputs["--mode rpc --no-session"] = []byte(`{"type":"auto_compaction_start"}`)
	blocked := ProbeOMPContextCapabilities(context.Background(), OMPContextCapabilityOptions{
		Runner: versionOnly, Timeout: time.Second, MaxOutput: 4096, RequestedHistoryMode: "active", RequestedMemoryMode: "shadow",
	})
	require.False(t, blocked.ActiveEligible)
	require.Equal(t, "shadow", blocked.EffectiveHistoryMode)
	require.Equal(t, "off", blocked.EffectiveMemoryMode)
	require.Equal(t, "event_missing", ompContextCapabilityByID(t, blocked, "lifecycle.post_compaction").Reason)
}

func TestEffectiveOMPContextMemoryMode_RequiresShadowEligibility(t *testing.T) {
	t.Parallel()

	require.Equal(t, "shadow", effectiveOMPContextMemoryMode("shadow", true))
	require.Equal(t, "off", effectiveOMPContextMemoryMode("shadow", false))
}

func cloneOMPContextOutputs(input map[string][]byte) map[string][]byte {
	result := make(map[string][]byte, len(input))
	for key, value := range input {
		result[key] = append([]byte(nil), value...)
	}
	return result
}

func ompContextCapabilityByID(t *testing.T, report OMPContextCapabilityReport, id string) OMPContextCapability {
	t.Helper()
	for _, capability := range report.Capabilities {
		if capability.ID == id {
			return capability
		}
	}
	t.Fatalf("missing capability %s", id)
	return OMPContextCapability{}
}

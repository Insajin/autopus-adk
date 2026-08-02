package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
)

type ompContextDoctorFakeRunner struct {
	outputs map[string][]byte
	errors  map[string]error
	calls   []string
}

func (runner *ompContextDoctorFakeRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	runner.calls = append(runner.calls, key)
	return append([]byte(nil), runner.outputs[key]...), runner.errors[key]
}

func TestProbeOMPContextCurrentRuntime_UsesOnlyLocalIdentityAndConfigList(t *testing.T) {
	t.Parallel()

	runner := &ompContextDoctorFakeRunner{outputs: map[string][]byte{
		"--version":          []byte("omp/17.1.8\n"),
		"config list --json": []byte(`{"settings":{"compaction":{"enabled":true},"memory":{"enabled":false}}}`),
	}, errors: map[string]error{}}
	probe := probeOMPContextCurrentRuntime(context.Background(), runner)
	assert.True(t, probe.IdentityVerified)
	assert.True(t, probe.ConfigListSchema)
	assert.True(t, probe.CompactionSchema)
	assert.True(t, probe.MemorySchema)
	assert.True(t, probe.OverlayReadback)
	assert.Equal(t, []string{"--version", "config list --json"}, runner.calls)
	for _, call := range runner.calls {
		assert.NotContains(t, call, "rpc")
		assert.NotContains(t, call, "prompt")
		assert.NotContains(t, call, "run")
	}
}

func TestProbeOMPContextCurrentRuntime_InvalidSchemaAndFailuresFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		outputs map[string][]byte
		errors  map[string]error
		reason  string
	}{
		{name: "version", outputs: map[string][]byte{"--version": []byte("omp/latest")}, errors: map[string]error{}, reason: "identity_unverified"},
		{name: "schema", outputs: map[string][]byte{"--version": []byte("omp/17.1.8"), "config list --json": []byte(`{"theme":"dark"}`)}, errors: map[string]error{}, reason: "config_schema_unproved"},
		{name: "command", outputs: map[string][]byte{"--version": []byte("omp/17.1.8")}, errors: map[string]error{"config list --json": errors.New("denied")}, reason: "config_readback_unproved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &ompContextDoctorFakeRunner{outputs: tt.outputs, errors: tt.errors}
			probe := probeOMPContextCurrentRuntime(context.Background(), runner)
			assert.Equal(t, tt.reason, probe.Reason)
		})
	}
}

func TestBuildOMPContextDoctorInput_NoOptInDoesNotProbe(t *testing.T) {
	t.Parallel()

	runner := &ompContextDoctorFakeRunner{outputs: map[string][]byte{}, errors: map[string]error{}}
	input := buildOMPContextDoctorInput(context.Background(), t.TempDir(), config.DefaultFullConfig("no-opt"), runner)
	assert.False(t, input.Enabled)
	assert.Empty(t, runner.calls)
}

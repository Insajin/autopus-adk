package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ompModelDoctorFakeRunner struct {
	outputs map[string][]byte
	errors  map[string]error
	calls   []string
}

func (runner *ompModelDoctorFakeRunner) Run(
	_ context.Context,
	_ string,
	args ...string,
) ([]byte, error) {
	key := strings.Join(args, " ")
	runner.calls = append(runner.calls, key)
	return append([]byte(nil), runner.outputs[key]...), runner.errors[key]
}

func TestBuildOMPModelDoctorInput_EnrichesActualAvailableCatalogWithoutModelCall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	available := []byte(`{"models":[{"provider":"acme","id":"model","selector":"acme/model","reasoning":true,"input":["text","image"],"thinking":["high"]}]}`)
	cfg := ompDoctorPolicyConfig("acme")
	declarations := make([]omp.OMPModelCatalogDeclaration, 0, len(config.OMPProviderNeutralCapabilities()))
	for _, capability := range config.OMPProviderNeutralCapabilities() {
		declarations = append(declarations, omp.OMPModelCatalogDeclaration{
			Selector: "acme/model", Family: "acme", Capability: capability,
		})
	}
	catalog, reason := omp.NormalizeOMPAvailableCatalog(available, ompModelDoctorProbeOutput, declarations)
	require.Equal(t, "catalog_ready", reason)
	expectation, err := omp.CompileOMPModelDoctorActivationExpectation(cfg.RoleModelPolicy.Profiles["balanced"], catalog)
	require.NoError(t, err)
	overlay := expectation.ConfigBytes
	overlayPath := filepath.Join(root, filepath.FromSlash(omp.DefaultOMPModelOverlayPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(overlayPath), 0o700))
	require.NoError(t, os.WriteFile(overlayPath, overlay, 0o600))
	canonicalOverlayPath, err := filepath.EvalSymlinks(overlayPath)
	require.NoError(t, err)
	runner := &ompModelDoctorFakeRunner{outputs: map[string][]byte{
		"--version":     []byte("omp/17.1.8\n"),
		"models --json": available,
	}, errors: map[string]error{}}
	for key, value := range expectation.ExpectedValues {
		readback, marshalErr := json.Marshal(map[string]any{"key": key, "value": value})
		require.NoError(t, marshalErr)
		runner.outputs["--config "+canonicalOverlayPath+" config get "+key+" --json"] = readback
	}

	input := buildOMPModelDoctorInput(context.Background(), root, cfg, runner)
	assert.True(t, input.Enabled)
	assert.Equal(t, "ready", input.Probe.Status)
	assert.Equal(t, "catalog_ready", input.Probe.Reason)
	assert.Equal(t, "omp/17.1.8", input.Probe.Version)
	assert.Equal(t, omp.OMPModelSHA256(overlay), input.Activation.ConfigHash)
	canonicalReadback, err := json.Marshal(expectation.ExpectedValues)
	require.NoError(t, err)
	assert.Equal(t, omp.OMPModelSHA256(canonicalReadback), input.Activation.ReadbackHash)
	assert.Len(t, input.Compilation.Resolutions, 16)
	for _, resolution := range input.Compilation.Resolutions {
		assert.Equal(t, "selected", resolution.Status, resolution.RouteID)
		assert.Equal(t, "availability", resolution.EvidenceClass, resolution.RouteID)
		assert.False(t, resolution.QuorumEvidence, resolution.RouteID)
	}
	assert.Equal(t, []string{
		"--version", "models --json", "models --json",
		"--config " + canonicalOverlayPath + " config get modelRoles --json",
		"--config " + canonicalOverlayPath + " config get retry.fallbackChains --json",
		"--config " + canonicalOverlayPath + " config get retry.modelFallback --json",
	}, runner.calls)
	for _, call := range runner.calls {
		assert.NotContains(t, call, "prompt")
	}
}

func TestBuildOMPModelDoctorInput_MetadataGapStaysExactBlockedReason(t *testing.T) {
	t.Parallel()

	runner := &ompModelDoctorFakeRunner{outputs: map[string][]byte{
		"--version":     []byte("omp/17.1.8\n"),
		"models --json": []byte(`{"models":[{"provider":"acme","id":"model","selector":"acme/model"}]}`),
	}, errors: map[string]error{}}
	cfg := ompDoctorPolicyConfig("")
	input := buildOMPModelDoctorInput(context.Background(), t.TempDir(), cfg, runner)
	assert.True(t, input.Enabled)
	assert.Equal(t, "blocked", input.Probe.Status)
	assert.Equal(t, "catalog_metadata_insufficient", input.Probe.Reason)
}

func TestBuildOMPModelDoctorInput_NoSelectedProfileDoesNotProbe(t *testing.T) {
	t.Parallel()

	runner := &ompModelDoctorFakeRunner{outputs: map[string][]byte{}, errors: map[string]error{}}
	input := buildOMPModelDoctorInput(context.Background(), t.TempDir(), config.DefaultFullConfig("no-opt"), runner)
	assert.False(t, input.Enabled)
	assert.Empty(t, runner.calls)
}

func TestReadOMPModelDoctorActivation_RejectsSymlinkedOverlayParent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".autopus"), 0o700))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, ".autopus", "runtime")))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "omp-model-routing-v1.yml"), []byte("outside"), 0o600))
	runner := &ompModelDoctorFakeRunner{outputs: map[string][]byte{}, errors: map[string]error{}}
	evidence := readOMPModelDoctorActivation(context.Background(), root, runner)
	assert.Empty(t, evidence.ConfigHash)
	assert.Empty(t, evidence.ReadbackHash)
	assert.Empty(t, runner.calls)
}

func TestBuildOMPModelDoctorInput_StrictCatalogAndFailureBranches(t *testing.T) {
	t.Parallel()

	assert.False(t, buildOMPModelDoctorInput(context.Background(), t.TempDir(), nil, nil).Enabled)
	missingRunner := buildOMPModelDoctorInput(
		context.Background(), t.TempDir(), ompDoctorPolicyConfig("acme"), nil,
	)
	assert.Equal(t, "identity_unverified", missingRunner.Probe.Reason)

	rich := &ompModelDoctorFakeRunner{outputs: map[string][]byte{
		"--version":     []byte("omp/17.1.8\n"),
		"models --json": []byte(`{"models":[{"provider":"acme","id":"model","family":"acme","capabilities":["coding_tool_use","deep_reasoning","deterministic_transform","fast_validation","independent_dissent","vision_design"],"thinking":["high"],"auth_enabled":true}]}`),
	}, errors: map[string]error{}}
	strict := buildOMPModelDoctorInput(context.Background(), t.TempDir(), ompDoctorPolicyConfig("acme"), rich)
	assert.Equal(t, "ready", strict.Probe.Status)
	assert.Equal(t, []string{"--version", "models --json"}, rich.calls)
	assert.Empty(t, strict.Activation.ConfigHash)

	failed := &ompModelDoctorFakeRunner{outputs: map[string][]byte{
		"--version": []byte("omp/17.1.8\n"),
	}, errors: map[string]error{"models --json": errors.New("catalog unavailable")}}
	blocked := buildOMPModelDoctorInput(context.Background(), t.TempDir(), ompDoctorPolicyConfig("acme"), failed)
	assert.Equal(t, "blocked", blocked.Probe.Status)
	assert.Equal(t, "catalog_invalid", blocked.Probe.Reason)
}

func TestOMPModelDoctorExecRunner_LocalBoundedCommandAndNoOptProbe(t *testing.T) {
	t.Parallel()

	runner := ompModelDoctorExecRunner{}
	_, err := runner.Run(context.Background(), "go", "version")
	require.ErrorContains(t, err, "unsafe OMP model doctor command")

	root := t.TempDir()
	require.NoError(t, config.Save(root, config.DefaultFullConfig("no-opt-doctor")))
	report := probeOMPModelRoutingDoctor(context.Background(), root)
	assert.False(t, report.Enabled)
}

func TestReadOMPModelDoctorActivation_InvalidModeAndReadbackFailureFailClosed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(omp.DefaultOMPModelOverlayPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("modelRoles: {}\n"), 0o644))
	runner := &ompModelDoctorFakeRunner{outputs: map[string][]byte{}, errors: map[string]error{}}
	assert.Empty(t, readOMPModelDoctorActivation(context.Background(), root, runner).ConfigHash)

	require.NoError(t, os.Chmod(path, 0o600))
	runner.errors["--config "+omp.DefaultOMPModelOverlayPath+" config get modelRoles"] = errors.New("readback")
	evidence := readOMPModelDoctorActivation(context.Background(), root, runner)
	assert.Equal(t, omp.OMPModelSHA256([]byte("modelRoles: {}\n")), evidence.ConfigHash)
	assert.Empty(t, evidence.ReadbackHash)
}

func ompDoctorPolicyConfig(family string) *config.HarnessConfig {
	cfg := config.DefaultFullConfig("doctor-routing")
	routes := make(map[string]config.RoleCapabilityRouteConf)
	for _, capability := range config.OMPProviderNeutralCapabilities() {
		routes[capability] = config.RoleCapabilityRouteConf{
			Required: true,
			Candidates: []config.RoleModelCandidateConf{{
				Selector: "acme/model", Thinking: "high", Family: family,
			}},
		}
	}
	cfg.RoleModelPolicy = config.RoleModelPolicyConf{
		Version: config.RoleModelPolicyVersionV1, Profile: "balanced",
		Profiles: map[string]config.RoleModelProfileConf{
			"balanced": {ConfigMode: config.RoleModelConfigModeOverlay, Capabilities: routes},
		},
	}
	return cfg
}

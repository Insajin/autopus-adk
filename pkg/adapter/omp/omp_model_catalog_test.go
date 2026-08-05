package omp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type modelCatalogFakeRunner struct {
	outputs map[string][]byte
	errors  map[string]error
	calls   []string
}

func (runner *modelCatalogFakeRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	runner.calls = append(runner.calls, key)
	return runner.outputs[key], runner.errors[key]
}

func TestProbeOMPModelCatalog_WithAllowedMetadata_ReturnsCanonicalSecretFreeFingerprint(t *testing.T) {
	t.Parallel()
	runner := &modelCatalogFakeRunner{outputs: map[string][]byte{
		"--version":                    []byte("omp/17.2.6\n"),
		"config get modelRoles --json": []byte(`{"key":"modelRoles","value":{}}`),
		"models --json --no-extensions": []byte(`{"models":[
			{"provider":"openai","id":"beta-coder","family":"openai","capabilities":["fast_validation","coding_tool_use"],"thinking":["high","medium"],"auth_enabled":true,"token":"never-fingerprint"},
			{"provider":"anthropic","id":"alpha-reasoner","family":"anthropic","capabilities":["independent_dissent","deep_reasoning"],"thinking":["max","xhigh","high","off","none","auto"],"available":true}
		]}`),
	}, errors: map[string]error{}}

	got := ProbeOMPModelCatalog(context.Background(), OMPModelCatalogProbeOptions{
		Executable: "omp", Runner: runner, Timeout: time.Second, MaxOutput: 4096,
		Settings: []string{"modelRoles"},
	})

	require.Equal(t, "ready", got.Status)
	require.Equal(t, "catalog_ready", got.Reason)
	require.Equal(t, "omp/17.2.6", got.Version)
	require.Regexp(t, `^sha256:[0-9a-f]{64}$`, got.Catalog.Fingerprint)
	require.Equal(t, []string{"anthropic/alpha-reasoner", "openai/beta-coder"}, []string{
		got.Catalog.Models[0].Provider + "/" + got.Catalog.Models[0].Model,
		got.Catalog.Models[1].Provider + "/" + got.Catalog.Models[1].Model,
	})
	require.Equal(t, []string{"deep_reasoning", "independent_dissent"}, got.Catalog.Models[0].Capabilities)
	require.Equal(t, []string{"auto", "high", "max", "none", "off", "xhigh"}, got.Catalog.Models[0].Thinking)
	require.Equal(t, []OMPModelSettingSupport{{Key: "modelRoles", Supported: true, Reason: "observed"}}, got.Settings)

	withoutSecret := strings.ReplaceAll(string(runner.outputs["models --json --no-extensions"]), `,"token":"never-fingerprint"`, "")
	wantCatalog, reason := NormalizeOMPModelCatalog([]byte(withoutSecret), 4096)
	require.Equal(t, "catalog_ready", reason)
	require.Equal(t, got.Catalog.Fingerprint, wantCatalog.Fingerprint)
}

func TestProbeOMPModelCatalog_WithInvalidIdentity_StopsBeforeSettingsAndCatalog(t *testing.T) {
	t.Parallel()
	runner := &modelCatalogFakeRunner{outputs: map[string][]byte{"--version": []byte("other/17.1.8")}, errors: map[string]error{}}

	got := ProbeOMPModelCatalog(context.Background(), OMPModelCatalogProbeOptions{
		Runner: runner, Timeout: time.Second, MaxOutput: 1024, Settings: []string{"modelRoles"},
	})

	require.Equal(t, "blocked", got.Status)
	require.Equal(t, "identity_unverified", got.Reason)
	require.Equal(t, []string{"--version"}, runner.calls)
}

func TestNormalizeOMPModelCatalog_WithFailureFixtures_ReturnsExactReasons(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  []byte
		max  int
		want string
	}{
		{name: "empty", raw: []byte(`{"models":[]}`), max: 1024, want: "catalog_empty"},
		{name: "malformed", raw: []byte(`{"models":[`), max: 1024, want: "catalog_invalid"},
		{name: "missing models", raw: []byte(`{"secret":"sentinel"}`), max: 1024, want: "catalog_invalid"},
		{name: "missing semantic metadata", raw: []byte(`{"models":[{"provider":"p","id":"m","available":true}]}`), max: 1024, want: "catalog_metadata_insufficient"},
		{name: "trailing garbage", raw: []byte(`{"models":[]}garbage`), max: 1024, want: "catalog_invalid"},
		{name: "oversized", raw: []byte(`{"models":[]}`), max: 4, want: "catalog_oversized"},
		{name: "duplicate", raw: []byte(`{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":["high"],"keyless":true},{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":["high"],"keyless":true}]}`), max: 1024, want: "catalog_invalid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, reason := NormalizeOMPModelCatalog(tc.raw, tc.max)
			require.Equal(t, tc.want, reason)
			require.Empty(t, got.Models)
		})
	}
}

func TestProbeOMPModelCatalog_WithBoundFailures_ReturnsCatalogReasons(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		output []byte
		err    error
		want   string
	}{
		{name: "timeout", err: context.DeadlineExceeded, want: "catalog_timeout"},
		{name: "oversized", output: []byte(strings.Repeat("x", 65)), want: "catalog_oversized"},
		{name: "invalid", output: []byte(`{"models":`), want: "catalog_invalid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := &modelCatalogFakeRunner{outputs: map[string][]byte{
				"--version": []byte("omp/17.2.6"), "models --json --no-extensions": tc.output,
			}, errors: map[string]error{"models --json --no-extensions": tc.err}}
			got := ProbeOMPModelCatalog(context.Background(), OMPModelCatalogProbeOptions{
				Runner: runner, Timeout: time.Second, MaxOutput: 64,
			})
			require.Equal(t, "blocked", got.Status)
			require.Equal(t, tc.want, got.Reason)
		})
	}
}

func TestProbeOMPModelCatalog_WithUnsupportedSetting_ReportsItWithoutReadingUnknownKeys(t *testing.T) {
	t.Parallel()
	runner := &modelCatalogFakeRunner{outputs: map[string][]byte{
		"--version": []byte("omp/17.2.6"), "models --json --no-extensions": []byte(`{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":["high"],"keyless":true}]}`),
	}, errors: map[string]error{"config get retry.fallbackChains --json": errors.New("unsupported")}}

	got := ProbeOMPModelCatalog(context.Background(), OMPModelCatalogProbeOptions{
		Runner: runner, Timeout: time.Second, MaxOutput: 1024,
		Settings: []string{"retry.fallbackChains", "credential.token"},
	})

	require.Equal(t, "ready", got.Status)
	require.Equal(t, []OMPModelSettingSupport{
		{Key: "credential.token", Reason: "setting_not_allowlisted"},
		{Key: "retry.fallbackChains", Reason: "exit_nonzero"},
	}, got.Settings)
	require.NotContains(t, runner.calls, "config get credential.token --json")
}

func TestNormalizeOMPAvailableCatalog_ProfileDeclarationsAreNotRuntimeEvidence(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"models":[{"provider":"p","id":"reasoner","selector":"p/reasoner","reasoning":true,"input":["text","image"],"thinking":["max","high"],"apiKey":"discard"}]}`)
	got, reason := NormalizeOMPAvailableCatalog(raw, 4096, []OMPModelCatalogDeclaration{
		{Selector: "p/reasoner", Family: "family-p", Capability: "deep_reasoning"},
	})

	require.Equal(t, "catalog_metadata_insufficient", reason)
	require.Empty(t, got.Models)
	require.Empty(t, got.Fingerprint)
}

func TestSafeOMPModelToken_RejectsStructuralAndControlCharacters(t *testing.T) {
	t.Parallel()

	require.True(t, safeOMPModelToken("model-v1"))
	for _, value := range []string{"", " model", "provider/model", "model\x1f", "model\x7f"} {
		require.False(t, safeOMPModelToken(value), value)
	}
}

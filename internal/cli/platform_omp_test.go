package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

type ompCLIJSONEnvelope struct {
	Status jsonEnvelopeStatus `json:"status"`
	Data   json.RawMessage    `json:"data"`
}

type ompCLIFakeRunner struct {
	catalog []byte
	fail    bool
	calls   []string
}

func (runner *ompCLIFakeRunner) Run(_ context.Context, executable string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, executable+" "+strings.Join(args, " "))
	if runner.fail {
		return nil, errors.New("fixture unavailable")
	}
	switch strings.Join(args, " ") {
	case "--version":
		return []byte("omp/17.1.8\n"), nil
	case "models --json --no-extensions":
		return append([]byte(nil), runner.catalog...), nil
	case "config list --json":
		return []byte(`{"compaction":{},"memory":{}}`), nil
	default:
		if len(args) >= 4 && args[len(args)-1] == "--json" && args[len(args)-3] == "config" && args[len(args)-2] == "get" {
			return []byte(`{}`), nil
		}
		return nil, fmt.Errorf("unexpected fixture args: %s", strings.Join(args, " "))
	}
}

func TestPlatformOMPHelpSurfacesCommandsAndExactFlags(t *testing.T) {
	assertCommandHelpContains(t, []string{"platform", "omp", "--help"}, "models", "profile", "explain", "--dir")
	assertCommandHelpContains(t, []string{"platform", "omp", "models", "--help"}, "--json", "--format")
	assertCommandHelpContains(t, []string{"platform", "omp", "profile", "init", "--help"}, "--name", "--plan", "--json", "--format")
	assertCommandHelpContains(t, []string{"platform", "omp", "profile", "apply", "--help"}, "apply <name>", "--json", "--format")
	assertCommandHelpContains(t, []string{"platform", "omp", "explain", "--help"}, "--json", "--format")
	assertCommandHelpContains(t, []string{"status", "--help"}, "--platform", "--dir", "--json", "--format")
}

func TestPlatformOMPModelsReadyTextAndJSON(t *testing.T) {
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	dir := t.TempDir()
	deps := ompPlatformDependencies{newRunner: func() omp.OMPModelCatalogRunner { return runner }}

	text := executeOMPSubcommand(t, newPlatformOMPModelsCmd(&dir, normalizeOMPPlatformDependencies(deps)))
	assert.Contains(t, text, "OMP installed model catalog")
	assert.Contains(t, text, "anthropic/alpha-reasoner")
	assert.NotContains(t, strings.ToLower(text), "secret")

	encoded := executeOMPSubcommand(t, newPlatformOMPModelsCmd(&dir, normalizeOMPPlatformDependencies(deps)), "--json")
	var envelope ompCLIJSONEnvelope
	require.NoError(t, json.Unmarshal([]byte(encoded), &envelope))
	assert.Equal(t, jsonStatusOK, envelope.Status)
	var payload ompCatalogPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.Equal(t, "catalog_ready", payload.Reason)
	assert.Len(t, payload.Models, 5)
}

func TestPlatformOMPProfileInitPlanWritesNothingAndHasSixCapabilities(t *testing.T) {
	root := t.TempDir()
	sentinelPath := filepath.Join(root, "autopus.yaml")
	sentinel := []byte("sentinel: unchanged\n")
	require.NoError(t, os.WriteFile(sentinelPath, sentinel, 0o640))
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})

	encoded := executeOMPSubcommand(
		t, newPlatformOMPProfileInitCmd(&root, deps), "--name", "balanced", "--plan", "--json",
	)
	var envelope ompCLIJSONEnvelope
	require.NoError(t, json.Unmarshal([]byte(encoded), &envelope))
	var payload ompProfilePlanPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.Equal(t, "plan", payload.Mode)
	assert.Empty(t, payload.Writes)
	assert.Len(t, payload.Capabilities, 6)
	after, err := os.ReadFile(sentinelPath)
	require.NoError(t, err)
	assert.Equal(t, sentinel, after)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestPlatformOMPProfileApplyPersistsAndRollsBackAtomically(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultFullConfig("omp-apply")
	cfg.Platforms = []string{"omp"}
	require.NoError(t, config.Save(root, cfg))
	configPath := filepath.Join(root, "autopus.yaml")
	original, err := os.ReadFile(configPath)
	require.NoError(t, err)
	original = append(original, []byte("operator_extension:\n  credential_ref: ${OMP_SECRET}\n")...)
	require.NoError(t, os.WriteFile(configPath, original, 0o640))
	require.NoError(t, os.Chmod(configPath, 0o640))
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	activated := false
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
		activate: func(_ context.Context, _ string, applied *config.HarnessConfig) error {
			activated = true
			name, profile, ok := applied.RoleModelPolicy.SelectedRoleModelProfile()
			require.True(t, ok)
			assert.Equal(t, "balanced", name)
			return preflightOMPProfile(name, profile, mustOMPCatalog(t, runner.catalog))
		},
	})
	dir := root
	executeOMPSubcommand(t, newPlatformOMPProfileApplyCmd(&dir, deps), "balanced")
	assert.True(t, activated)
	loaded, err := config.LoadPreview(root)
	require.NoError(t, err)
	name, _, selected := loaded.RoleModelPolicy.SelectedRoleModelProfile()
	assert.True(t, selected)
	assert.Equal(t, "balanced", name)
	persisted, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(persisted), "credential_ref: ${OMP_SECRET}")
	assert.Equal(t, os.FileMode(0o640), mustFileMode(t, configPath))

	before, err := os.ReadFile(configPath)
	require.NoError(t, err)
	_, err = applyOMPProfile(
		context.Background(), root, "second", runner,
		func(context.Context, string, *config.HarnessConfig) error { return errors.New("activation failed") },
	)
	require.Error(t, err)
	after, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, before, after)
}

func TestPlatformOMPExplainShowsDisabledUnauthorizedAndMissingFallbacks(t *testing.T) {
	root, runner, profile := writeSelectedOMPProfile(t)
	route := profile.Capabilities[config.CapabilityCodingToolUse]
	route.Candidates = []config.RoleModelCandidateConf{
		{Selector: "openai/disabled-coder", Thinking: "high", Family: "openai"},
		{Selector: "openai/unauthorized-coder", Thinking: "high", Family: "openai"},
		{Selector: "openai/missing-coder", Thinking: "high", Family: "openai"},
		{Selector: "openai/beta-coder", Thinking: "high", Family: "openai"},
	}
	profile.Capabilities[config.CapabilityCodingToolUse] = route
	cfg, err := config.LoadPreview(root)
	require.NoError(t, err)
	cfg.RoleModelPolicy.Profiles["balanced"] = profile
	require.NoError(t, config.Save(root, cfg))
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})
	dir := root

	text := executeOMPSubcommand(t, newPlatformOMPExplainCmd(&dir, deps))
	for _, reason := range []string{"disabled", "unauthorized", "model_unknown", "selected"} {
		assert.Contains(t, text, "reason="+reason)
	}
	encoded := executeOMPSubcommand(t, newPlatformOMPExplainCmd(&dir, deps), "--json")
	var explainEnvelope struct {
		Data struct {
			Models ompModelOperatorProjection `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(encoded), &explainEnvelope))
	reasons := map[string]bool{}
	for _, model := range explainEnvelope.Data.Models.Models {
		for _, attempt := range model.FallbackAttempts {
			reasons[attempt.Reason] = true
		}
	}
	for _, reason := range []string{"disabled", "unauthorized", "model_unknown"} {
		assert.True(t, reasons[reason], reason)
	}
	assert.NotContains(t, strings.ToLower(encoded), "api_key")
}

func TestOMPStatusProjectionReportsStaleCatalogAndMissingReceipt(t *testing.T) {
	root, runner, _ := writeSelectedOMPProfile(t)
	missing := buildOMPPlatformProjection(context.Background(), root, runner, time.Now().UTC())
	assert.Equal(t, "blocked", missing.Models.Status)
	assert.Equal(t, "receipt_missing", missing.Models.Reason)
	assert.Contains(t, missing.Blockers, "models:receipt_missing")
	var missingText bytes.Buffer
	missingTextCmd := &cobra.Command{Use: "status"}
	missingTextCmd.SetOut(&missingText)
	require.NoError(t, renderOMPPlatformStatus(missingTextCmd, missing, false))
	assert.Contains(t, missingText.String(), "reason=receipt_missing")

	var missingJSON bytes.Buffer
	missingJSONCmd := &cobra.Command{Use: "status"}
	missingJSONCmd.SetOut(&missingJSON)
	require.NoError(t, renderOMPPlatformStatus(missingJSONCmd, missing, true))
	var missingEnvelope struct {
		Data ompPlatformProjection `json:"data"`
	}
	require.NoError(t, json.Unmarshal(missingJSON.Bytes(), &missingEnvelope))
	assert.Equal(t, "receipt_missing", missingEnvelope.Data.Models.Reason)

	writeStaleOMPModelReceipt(t, root)
	stale := buildOMPPlatformProjection(context.Background(), root, runner, time.Now().UTC())
	assert.Equal(t, "catalog_stale", stale.Models.Reason)
	assert.False(t, stale.Models.ReceiptVerified)
	assert.Contains(t, stale.Blockers, "models:catalog_stale")
	var staleText bytes.Buffer
	staleTextCmd := &cobra.Command{Use: "status"}
	staleTextCmd.SetOut(&staleText)
	require.NoError(t, renderOMPPlatformStatus(staleTextCmd, stale, false))
	assert.Contains(t, staleText.String(), "reason=catalog_stale")

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "status"}
	cmd.SetOut(&out)
	require.NoError(t, renderOMPPlatformStatus(cmd, stale, true))
	var statusEnvelope struct {
		Data ompPlatformProjection `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &statusEnvelope))
	assert.Equal(t, "catalog_stale", statusEnvelope.Data.Models.Reason)
	assert.False(t, statusEnvelope.Data.ReceiptVerification.ModelVerified)
}

func TestOMPProfileProposalRejectsInvalidMissingCapability(t *testing.T) {
	catalog := mustOMPCatalog(t, ompCLIReadyCatalogJSON())
	filtered := catalog.Models[:0]
	for _, model := range catalog.Models {
		if !containsOMPString(model.Capabilities, config.CapabilityVisionDesign) {
			filtered = append(filtered, model)
		}
	}
	catalog.Models = filtered
	_, err := buildOMPProfileProposal("balanced", catalog)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capability_unavailable:vision_design")
}

func TestOMPProfileInvalidNameFailsClosedInTextAndJSON(t *testing.T) {
	root := t.TempDir()
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})
	textCmd := newPlatformOMPProfileInitCmd(&root, deps)
	textCmd.SilenceErrors = true
	textCmd.SilenceUsage = true
	textCmd.SetArgs([]string{"--name", "invalid/name"})
	err := textCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "profile_name_invalid")
	assert.Empty(t, runner.calls)

	jsonCmd := newPlatformOMPProfileInitCmd(&root, deps)
	var out bytes.Buffer
	jsonCmd.SetOut(&out)
	jsonCmd.SetErr(&out)
	jsonCmd.SilenceErrors = true
	jsonCmd.SilenceUsage = true
	jsonCmd.SetArgs([]string{"--name", "invalid/name", "--json"})
	err = jsonCmd.Execute()
	require.Error(t, err)
	var envelope struct {
		Status jsonEnvelopeStatus `json:"status"`
		Error  jsonErrorPayload   `json:"error"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &envelope))
	assert.Equal(t, jsonStatusError, envelope.Status)
	assert.Equal(t, "omp_profile_invalid", envelope.Error.Code)
	assert.Contains(t, envelope.Error.Message, "profile_name_invalid")
	assert.Empty(t, runner.calls)
}

func TestOMPStatusNoOptInIsReadyHubLimitedAndBareStatusIsCompatible(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultFullConfig("omp-status")
	cfg.Platforms = []string{"omp"}
	require.NoError(t, config.Save(root, cfg))
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
		now:       func() time.Time { return time.Now().UTC() },
	})
	status := newStatusCmdWithOMPDependencies(deps)
	var operatorOut bytes.Buffer
	status.SetOut(&operatorOut)
	status.SetErr(&operatorOut)
	status.SetArgs([]string{"--dir", root, "--platform", "omp"})
	require.NoError(t, status.Execute())
	assert.Contains(t, operatorOut.String(), "status=ready")
	assert.Contains(t, operatorOut.String(), ompLiveStateLimitation)
	assert.Contains(t, operatorOut.String(), ompLiveStateNextCommand)
	assert.Empty(t, runner.calls, "no opt-in must not probe OMP")
	jsonStatusCmd := newStatusCmdWithOMPDependencies(deps)
	var jsonOut bytes.Buffer
	jsonStatusCmd.SetOut(&jsonOut)
	jsonStatusCmd.SetErr(&jsonOut)
	jsonStatusCmd.SetArgs([]string{"--dir", root, "--platform", "omp", "--json"})
	require.NoError(t, jsonStatusCmd.Execute())
	var statusEnvelope ompCLIJSONEnvelope
	require.NoError(t, json.Unmarshal(jsonOut.Bytes(), &statusEnvelope))
	assert.Equal(t, jsonStatusOK, statusEnvelope.Status)
	var statusPayload ompPlatformProjection
	require.NoError(t, json.Unmarshal(statusEnvelope.Data, &statusPayload))
	assert.Equal(t, "ready", statusPayload.Status)
	assert.Equal(t, ompLiveStateLimitation, statusPayload.ChildRuntime.Limitation)
	assert.Empty(t, runner.calls, "JSON no opt-in must not probe OMP")

	var want bytes.Buffer
	legacy := &cobra.Command{Use: "status"}
	legacy.SetOut(&want)
	require.NoError(t, runStatusText(legacy, root))
	bare := newStatusCmdWithOMPDependencies(deps)
	var got bytes.Buffer
	bare.SetOut(&got)
	bare.SetErr(&got)
	bare.SetArgs([]string{"--dir", root})
	require.NoError(t, bare.Execute())
	assert.Equal(t, want.String(), got.String())
}

func TestOMPModelsMissingExecutableFailsClosedInJSON(t *testing.T) {
	root := t.TempDir()
	runner := &ompCLIFakeRunner{fail: true}
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})
	cmd := newPlatformOMPModelsCmd(&root, deps)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	require.Error(t, err)
	var envelope struct {
		Status jsonEnvelopeStatus `json:"status"`
		Data   ompCatalogPayload  `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &envelope))
	assert.Equal(t, jsonStatusError, envelope.Status)
	assert.Equal(t, "identity_unverified", envelope.Data.Reason)
}

func writeSelectedOMPProfile(t *testing.T) (string, *ompCLIFakeRunner, config.RoleModelProfileConf) {
	t.Helper()
	root := t.TempDir()
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	catalog := mustOMPCatalog(t, runner.catalog)
	proposal, err := buildOMPProfileProposal("balanced", catalog)
	require.NoError(t, err)
	cfg := config.DefaultFullConfig("omp-selected")
	cfg.Platforms = []string{"omp"}
	cfg.RoleModelPolicy = config.RoleModelPolicyConf{
		Version: config.RoleModelPolicyVersionV1, Profile: "balanced",
		Profiles: map[string]config.RoleModelProfileConf{"balanced": proposal.profile},
	}
	require.NoError(t, config.Save(root, cfg))
	return root, runner, proposal.profile
}

func writeStaleOMPModelReceipt(t *testing.T, root string) {
	t.Helper()
	hash := "sha256:" + strings.Repeat("a", 64)
	receipt := omp.OMPModelResolutionReceipt{
		OMPVersion: "omp/17.1.8", CatalogFingerprint: hash, Profile: "balanced", ConfigSource: "overlay",
		Activation: omp.OMPModelActivationReceipt{Argv: []string{"omp"}, ConfigHash: hash, ReadbackHash: hash},
		Roles: []omp.OMPModelRoleReceipt{{
			Agent: "executor", Profile: "balanced", ConfigSource: "overlay", RequestedRole: "task", EffectiveRole: "task",
			Capability: config.CapabilityCodingToolUse, Provider: "openai", Model: "beta-coder",
			Selector: "openai/beta-coder", Thinking: "high",
			FamilyDiversity: omp.OMPModelFamilyDiversityReceipt{Status: "not_applicable"}, SafetySource: "user_effective",
		}},
		GeneratedAt: time.Now().UTC(),
	}
	_, err := omp.WriteOMPModelResolutionReceipt(omp.OMPModelReceiptWriteInput{WorkspaceRoot: root, Receipt: receipt})
	require.NoError(t, err)
}

func mustOMPCatalog(t *testing.T, data []byte) omp.OMPModelCatalog {
	t.Helper()
	catalog, reason := omp.NormalizeOMPModelCatalog(data, 1<<20)
	require.Equal(t, "catalog_ready", reason)
	return catalog
}

func ompCLIReadyCatalogJSON() []byte {
	return []byte(`{"models":[
		{"provider":"anthropic","id":"alpha-reasoner","family":"anthropic","capabilities":["deep_reasoning","independent_dissent"],"thinking":["high","xhigh"],"auth_enabled":true,"keyless":false,"disabled":false},
		{"provider":"openai","id":"beta-coder","family":"openai","capabilities":["coding_tool_use","fast_validation","independent_dissent","deterministic_transform"],"thinking":["low","medium","high"],"auth_enabled":true,"keyless":false,"disabled":false},
		{"provider":"google","id":"gamma-vision","family":"google","capabilities":["vision_design"],"thinking":["high"],"auth_enabled":true,"keyless":false,"disabled":false},
		{"provider":"openai","id":"disabled-coder","family":"openai","capabilities":["coding_tool_use"],"thinking":["high"],"auth_enabled":true,"keyless":false,"disabled":true},
		{"provider":"openai","id":"unauthorized-coder","family":"openai","capabilities":["coding_tool_use"],"thinking":["high"],"auth_enabled":false,"keyless":false,"disabled":false}
	]}`)
}

func executeOMPSubcommand(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute(), out.String())
	return out.String()
}

func mustFileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func assertCommandHelpContains(t *testing.T, args []string, expected ...string) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	for _, value := range expected {
		assert.Contains(t, out.String(), value)
	}
}

package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func runQualityShowForTest(t *testing.T, dir string) string {
	t.Helper()
	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--config", filepath.Join(dir, "autopus.yaml"), "quality", "show"})
	require.NoError(t, root.Execute(), buf.String())
	return buf.String()
}

func TestQualityShowReportsOMPPolicyDisabledWhenNoProfileSelected(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")

	out := runQualityShowForTest(t, dir)

	assert.Contains(t, out, "quality.default = balanced")
	assert.Contains(t, out, "omp.role_model_policy.enabled = false")
	assert.NotContains(t, out, "omp.role_model_policy.profile =")
	assert.Contains(t, out, ompRoleModelPolicyIndependenceNote)
}

// A global balanced quality value must never read as an active OMP ultra
// selection, so the OMP block reports its own profile on its own lines.
func TestQualityShowSeparatesOMPUltraFromGlobalBalancedDefault(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.RoleModelPolicy = config.RoleModelPolicyConf{
		Version: config.RoleModelPolicyVersionV1,
		Profile: "ultra",
		Family:  "openai",
		Agents: map[string]config.RoleAgentOverrideConf{
			"debugger": {Candidates: []config.RoleModelCandidateConf{
				{Selector: "anthropic/claude-fable-5-1", Thinking: "max", Family: "anthropic"},
			}},
		},
	}
	require.NoError(t, config.Save(dir, cfg))

	out := runQualityShowForTest(t, dir)

	assert.Contains(t, out, "quality.default = balanced")
	assert.Contains(t, out, "omp.role_model_policy.enabled = true")
	assert.Contains(t, out, "omp.role_model_policy.profile = ultra")
	assert.Contains(t, out, "omp.role_model_policy.family = openai")
	assert.Contains(t, out, "omp.role_model_policy.config_mode = overlay")
	assert.Contains(t, out, "omp.role_model_policy.source = builtin")
	assert.Contains(t, out, "omp.role_model_policy.agent_overrides = debugger")
}

func TestQualityShowReportsDefaultFamilyAndExplicitProfileSource(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	profile, ok := config.BuiltinRoleModelProfile("balanced", cfg.Quality, "anthropic", config.RoleModelConfigModeProjectManaged)
	require.True(t, ok)
	cfg.RoleModelPolicy = config.RoleModelPolicyConf{
		Version: config.RoleModelPolicyVersionV1,
		Profile: "house",
		Profiles: map[string]config.RoleModelProfileConf{
			"house": profile,
		},
	}
	require.NoError(t, config.Save(dir, cfg))

	out := runQualityShowForTest(t, dir)

	assert.Contains(t, out, "omp.role_model_policy.profile = house")
	assert.Contains(t, out, "omp.role_model_policy.family = default")
	assert.Contains(t, out, "omp.role_model_policy.source = explicit_profile")
	assert.Contains(t, out, "omp.role_model_policy.agent_overrides = none")
	assert.Contains(t, out, "omp.role_model_policy.config_mode = project-managed")
}

// quality show only reads; selecting an OMP profile must not have rewritten
// quality.default, and showing must not rewrite anything either.
func TestQualityShowLeavesConfigUnchanged(t *testing.T) {
	dir := writeQualityTestConfig(t, "ultra")
	before := readAutopusConfigTree(t, dir)

	runQualityShowForTest(t, dir)

	assert.Equal(t, before, readAutopusConfigTree(t, dir))
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Equal(t, "ultra", cfg.Quality.Default)
	assert.Empty(t, cfg.RoleModelPolicy.Profile)
}

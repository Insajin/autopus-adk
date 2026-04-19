// Package e2e provides user-facing scenario-based E2E test infrastructure.
package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveEnv_AutoDetect_FindsGoEnv verifies that ResolveEnv auto-detects
// well-known Go environment variables (e.g., GOPATH, GOROOT) without user input.
func TestResolveEnv_AutoDetect_FindsGoEnv(t *testing.T) {
	t.Parallel()

	// Given: a scenario in a Go project directory
	opts := EnvResolveOptions{
		ProjectDir:     t.TempDir(),
		ScenarioEnv:    map[string]string{},
		NonInteractive: true,
	}

	// When: ResolveEnv is called
	env, err := ResolveEnv(opts)

	// Then: standard Go env vars are present
	require.NoError(t, err)
	assert.NotNil(t, env)
	// GOPATH or GOROOT should be auto-detected from the current process env
	_, hasGoPath := env["GOPATH"]
	_, hasGoRoot := env["GOROOT"]
	assert.True(t, hasGoPath || hasGoRoot, "expected at least one Go env var to be auto-detected")
}

// TestResolveEnv_SafeDefaults_AppliedForDB verifies that DATABASE_URL receives
// a safe default value (sqlite://test.db) when not set explicitly.
// S8: safe default for DATABASE_URL.
func TestResolveEnv_SafeDefaults_AppliedForDB(t *testing.T) {
	t.Parallel()

	// Given: no DATABASE_URL in environment or scenario overrides
	opts := EnvResolveOptions{
		ProjectDir:     t.TempDir(),
		ScenarioEnv:    map[string]string{},
		NonInteractive: true,
		RequiredVars:   []string{"DATABASE_URL"},
	}

	// When: ResolveEnv is called
	env, err := ResolveEnv(opts)

	// Then: DATABASE_URL is set to the safe default
	require.NoError(t, err)
	assert.Equal(t, "sqlite://test.db", env["DATABASE_URL"])
}

// TestResolveEnv_PerScenarioOverride_TakesPrecedence verifies that per-scenario
// env var overrides take precedence over auto-detected and safe default values.
func TestResolveEnv_PerScenarioOverride_TakesPrecedence(t *testing.T) {
	t.Parallel()

	// Given: a scenario that overrides DATABASE_URL
	opts := EnvResolveOptions{
		ProjectDir:     t.TempDir(),
		ScenarioEnv:    map[string]string{"DATABASE_URL": "postgres://custom:5432/testdb"},
		NonInteractive: true,
	}

	// When: ResolveEnv is called
	env, err := ResolveEnv(opts)

	// Then: the scenario-level override value is used
	require.NoError(t, err)
	assert.Equal(t, "postgres://custom:5432/testdb", env["DATABASE_URL"])
}

// TestResolveEnv_NonInteractive_SkipsPrompt verifies that in non-interactive
// (CI/TTY-less) mode, missing external API keys cause the scenario to be SKIP
// rather than prompting the user.
// S17: CI TTY detection — non-interactive fallback.
func TestResolveEnv_NonInteractive_SkipsPrompt(t *testing.T) {
	t.Parallel()

	// Given: a non-interactive environment missing an external API key
	opts := EnvResolveOptions{
		ProjectDir:     t.TempDir(),
		ScenarioEnv:    map[string]string{},
		NonInteractive: true,
		RequiredVars:   []string{"EXTERNAL_API_KEY"},
	}

	// When: ResolveEnv is called
	_, err := ResolveEnv(opts)

	// Then: an ErrSkipScenario error is returned with the variable name
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSkipScenario)
	assert.Contains(t, err.Error(), "EXTERNAL_API_KEY")
	assert.Contains(t, err.Error(), "non-interactive mode")
}

// TestResolveEnv_MergeOrder_CorrectLayering verifies the merge precedence:
// auto-detect < safe defaults < per-scenario override.
func TestResolveEnv_MergeOrder_CorrectLayering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		scenarioEnv   map[string]string
		expectedValue string
	}{
		{
			name:          "scenario override wins over safe default",
			scenarioEnv:   map[string]string{"DATABASE_URL": "custom://override"},
			expectedValue: "custom://override",
		},
		{
			name:          "safe default used when no override",
			scenarioEnv:   map[string]string{},
			expectedValue: "sqlite://test.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Given: env options with specified scenario-level env
			opts := EnvResolveOptions{
				ProjectDir:     t.TempDir(),
				ScenarioEnv:    tt.scenarioEnv,
				NonInteractive: true,
				RequiredVars:   []string{"DATABASE_URL"},
			}

			// When: ResolveEnv is called
			env, err := ResolveEnv(opts)

			// Then: DATABASE_URL matches expected precedence
			require.NoError(t, err)
			assert.Equal(t, tt.expectedValue, env["DATABASE_URL"])
		})
	}
}

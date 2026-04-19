package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- SyncScenarios edge cases ----

// TestSync_BothCustomAndDeleted_CustomPreserved verifies that when multiple
// scenario types coexist, custom ones are preserved and non-custom removed ones
// are deprecated independently.
func TestSync_BothCustomAndDeleted_CustomPreserved(t *testing.T) {
	t.Parallel()

	existing := &ScenarioSet{
		Scenarios: []Scenario{
			{ID: "init", Command: "auto init", Status: "active"},
			{ID: "removed-cmd", Command: "auto removed", Status: "active"},
			{ID: "custom-manual", Command: "auto smoke", Status: "active"},
		},
	}
	currentCommands := []Scenario{
		{ID: "init", Command: "auto init", Status: "active"},
	}

	updated, err := SyncScenarios(existing, currentCommands)
	require.NoError(t, err)

	statusByID := make(map[string]string)
	for _, s := range updated.Scenarios {
		statusByID[s.ID] = s.Status
	}

	assert.Equal(t, "active", statusByID["init"])
	assert.Equal(t, "deprecated", statusByID["removed-cmd"])
	assert.Equal(t, "active", statusByID["custom-manual"])
}

// TestSync_EmptyExisting_AllCommandsAdded verifies that syncing into an
// empty ScenarioSet adds all commands as new active scenarios.
func TestSync_EmptyExisting_AllCommandsAdded(t *testing.T) {
	t.Parallel()

	existing := &ScenarioSet{Scenarios: []Scenario{}}
	commands := []Scenario{
		{ID: "alpha", Command: "auto alpha"},
		{ID: "beta", Command: "auto beta"},
	}

	updated, err := SyncScenarios(existing, commands)
	require.NoError(t, err)
	require.Len(t, updated.Scenarios, 2)
	for _, s := range updated.Scenarios {
		assert.Equal(t, "active", s.Status)
	}
}

// TestSync_EmptyCommands_AllDeprecated verifies that passing an empty command
// list marks all existing non-custom scenarios as deprecated.
func TestSync_EmptyCommands_AllDeprecated(t *testing.T) {
	t.Parallel()

	existing := &ScenarioSet{
		Scenarios: []Scenario{
			{ID: "init", Command: "auto init", Status: "active"},
			{ID: "doctor", Command: "auto doctor", Status: "active"},
		},
	}

	updated, err := SyncScenarios(existing, []Scenario{})
	require.NoError(t, err)
	require.Len(t, updated.Scenarios, 2)
	for _, s := range updated.Scenarios {
		assert.Equal(t, "deprecated", s.Status)
	}
}

// ---- ResolveEnv edge cases ----

// TestResolveEnv_NilScenarioEnv_DoesNotPanic verifies that a nil ScenarioEnv
// map is handled gracefully without panic.
func TestResolveEnv_NilScenarioEnv_DoesNotPanic(t *testing.T) {
	t.Parallel()

	opts := EnvResolveOptions{
		ProjectDir:     t.TempDir(),
		ScenarioEnv:    nil,
		NonInteractive: true,
	}

	env, err := ResolveEnv(opts)

	require.NoError(t, err)
	assert.NotNil(t, env)
}

// TestResolveEnv_EnvExampleAndTestEnvMerge verifies that .env.example values
// are overridden by values in .autopus/test.env (layer precedence).
func TestResolveEnv_EnvExampleAndTestEnvMerge(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".env.example"),
		[]byte("FOO=from-example\n"),
		0o644,
	))
	autopusDir := filepath.Join(dir, ".autopus")
	require.NoError(t, os.MkdirAll(autopusDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(autopusDir, "test.env"),
		[]byte("FOO=from-test-env\n"),
		0o644,
	))

	opts := EnvResolveOptions{
		ProjectDir:     dir,
		ScenarioEnv:    map[string]string{},
		NonInteractive: true,
	}

	env, err := ResolveEnv(opts)

	require.NoError(t, err)
	assert.Equal(t, "from-test-env", env["FOO"])
}

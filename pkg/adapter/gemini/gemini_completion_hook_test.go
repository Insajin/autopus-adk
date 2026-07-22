package gemini

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testGeminiCompletionHookTarget  = ".gemini/hooks/autopus/hook-gemini-afteragent.sh"
	testGeminiCompletionHookCommand = `"${GEMINI_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"/.gemini/hooks/autopus/hook-gemini-afteragent.sh`
	testGeminiStopHookTarget        = ".gemini/hooks/autopus/hook-gemini-stop.sh"
	testGeminiStopHookCommand       = `"$(cd .. && pwd)/.gemini/hooks/autopus/hook-gemini-stop.sh"`
)

func TestGenerate_InstallsGeminiOwnedCompletionHookContract(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	generated, err := NewWithRoot(root, WithoutPluginInstall()).Generate(
		context.Background(), config.DefaultFullConfig("gemini-only"),
	)
	require.NoError(t, err)

	assertGeminiCompletionHookContracts(t, root)
	assertMappingContainsPath(t, generated.Files, testGeminiCompletionHookTarget)
	assertMappingContainsPath(t, generated.Files, testGeminiStopHookTarget)
	manifest, err := adapter.LoadManifest(root, adapterName)
	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Contains(t, manifest.Files, testGeminiCompletionHookTarget)
	assert.Contains(t, manifest.Files, testGeminiStopHookTarget)
	assert.NoDirExists(t, filepath.Join(root, ".claude"), "Gemini-only install must not depend on Claude assets")
}

func TestUpdate_RestoresCompletionHookAndPreservesUserConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a := NewWithRoot(root, WithoutPluginInstall())
	cfg := config.DefaultFullConfig("gemini-only")
	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	settingsPath := filepath.Join(root, ".gemini", "settings.json")
	settings := readJSONDocument(t, settingsPath)
	settings["userSetting"] = map[string]any{"preserve": true}
	settingsHooks := requireJSONObject(t, settings["hooks"])
	settingsHooks["UserEvent"] = []any{map[string]any{"command": "user-hook"}}
	writeJSONDocument(t, settingsPath, settings)

	agentsHooksPath := filepath.Join(root, ".agents", "hooks.json")
	agentsHooks := readJSONDocument(t, agentsHooksPath)
	agentsHooks["user"] = map[string]any{"enabled": true}
	writeJSONDocument(t, agentsHooksPath, agentsHooks)

	assetPath := filepath.Join(root, filepath.FromSlash(testGeminiCompletionHookTarget))
	require.NoError(t, os.WriteFile(assetPath, []byte("stale\n"), 0o600))
	require.NoError(t, os.Chmod(assetPath, 0o600))

	updated, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)

	assertGeminiCompletionHookContracts(t, root)
	assertMappingContainsPath(t, updated.Files, testGeminiCompletionHookTarget)
	assertMappingContainsPath(t, updated.Files, testGeminiStopHookTarget)
	manifest, err := adapter.LoadManifest(root, adapterName)
	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Contains(t, manifest.Files, testGeminiCompletionHookTarget)
	assert.Contains(t, manifest.Files, testGeminiStopHookTarget)
	updatedSettings := readJSONDocument(t, settingsPath)
	assert.Equal(t, map[string]any{"preserve": true}, updatedSettings["userSetting"])
	assert.Contains(t, requireJSONObject(t, updatedSettings["hooks"]), "UserEvent")
	assert.Contains(t, readJSONDocument(t, agentsHooksPath), "user")
	assert.NoDirExists(t, filepath.Join(root, ".claude"), "Gemini-only update must not create Claude assets")
}

func TestGenerate_RejectsSymlinkedCompletionHookParent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".gemini"), 0o755))
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".gemini", "hooks")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := NewWithRoot(root, WithoutPluginInstall()).Generate(
		context.Background(), config.DefaultFullConfig("gemini-only"),
	)
	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(outside, "autopus", "hook-gemini-afteragent.sh"))
	assert.NoFileExists(t, filepath.Join(outside, "autopus", "hook-gemini-stop.sh"))
}

func TestClean_RemovesGeminiCompletionHookAsset(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a := NewWithRoot(root, WithoutPluginInstall())
	_, err := a.Generate(context.Background(), config.DefaultFullConfig("gemini-only"))
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(root, filepath.FromSlash(testGeminiCompletionHookTarget)))
	assert.FileExists(t, filepath.Join(root, filepath.FromSlash(testGeminiStopHookTarget)))

	require.NoError(t, a.Clean(context.Background()))
	assert.NoFileExists(t, filepath.Join(root, filepath.FromSlash(testGeminiCompletionHookTarget)))
	assert.NoFileExists(t, filepath.Join(root, filepath.FromSlash(testGeminiStopHookTarget)))
}

func TestGenerate_AntigravityStopCommandUsesConfigParentInsideOuterGit(t *testing.T) {
	t.Parallel()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required for the nested repository contract")
	}
	outer := t.TempDir()
	require.NoError(t, exec.Command(gitPath, "init", "--quiet", outer).Run())
	root := filepath.Join(outer, "nested-project")
	require.NoError(t, os.MkdirAll(root, 0o755))
	_, err = NewWithRoot(root, WithoutPluginInstall()).Generate(
		context.Background(), config.DefaultFullConfig("nested-project"),
	)
	require.NoError(t, err)

	command := flatHookCommand(t, filepath.Join(root, ".agents", "hooks.json"), "autopus", "Stop")
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = filepath.Join(root, ".agents")
	cmd.Env = append(os.Environ(), "AUTOPUS_SESSION_ID=")
	stdout, err := cmd.Output()
	require.NoError(t, err, "Stop command must resolve from the hooks.json directory")
	assert.JSONEq(t, `{"decision":"stop"}`, string(stdout))
}

func assertGeminiCompletionHookContracts(t *testing.T, root string) {
	t.Helper()
	settingsCommand := groupedHookCommand(
		t, filepath.Join(root, ".gemini", "settings.json"), "hooks", "AfterAgent",
	)
	agentsCommand := flatHookCommand(t, filepath.Join(root, ".agents", "hooks.json"), "autopus", "Stop")
	assert.Equal(t, testGeminiCompletionHookCommand, settingsCommand)
	assert.Equal(t, testGeminiStopHookCommand, agentsCommand)
	assertGeminiManagedHookExecutable(t, root, testGeminiCompletionHookTarget)
	assertGeminiManagedHookExecutable(t, root, testGeminiStopHookTarget)
	assertLegacyGeminiHookRunsFromNestedDirectory(t, root, settingsCommand)
	assertAntigravityHookRunsFromAgentsDirectory(t, root, agentsCommand)
}

func assertGeminiManagedHookExecutable(t *testing.T, root, target string) {
	t.Helper()
	assetPath := filepath.Join(root, filepath.FromSlash(target))
	info, err := os.Stat(assetPath)
	require.NoError(t, err, "managed completion command target must exist")
	assert.True(t, info.Mode().IsRegular())
	assert.NotZero(t, info.Mode().Perm()&0o111, "managed completion command target must be executable")
}

func assertLegacyGeminiHookRunsFromNestedDirectory(t *testing.T, root, command string) {
	t.Helper()
	nested := filepath.Join(root, "nested", "work")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	binDir := t.TempDir()
	fakeGit := filepath.Join(binDir, "git")
	require.NoError(t, os.WriteFile(fakeGit, []byte("#!/bin/sh\nprintf '%s\\n' \"$GEMINI_TEST_PROJECT_ROOT\"\n"), 0o755))

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = nested
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GEMINI_TEST_PROJECT_ROOT="+root,
		"AUTOPUS_SESSION_ID=",
	)
	require.NoError(t, cmd.Run(), "project-root command must execute from a nested working directory")
}

func assertAntigravityHookRunsFromAgentsDirectory(t *testing.T, root, command string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = filepath.Join(root, ".agents")
	cmd.Env = append(os.Environ(), "AUTOPUS_SESSION_ID=")
	stdout, err := cmd.Output()
	require.NoError(t, err)
	assert.JSONEq(t, `{"decision":"stop"}`, string(stdout))
}

func groupedHookCommand(t *testing.T, path, container, event string) string {
	t.Helper()
	document := readJSONDocument(t, path)
	events := requireJSONObject(t, document[container])
	entries, ok := events[event].([]any)
	require.True(t, ok)
	require.NotEmpty(t, entries)
	entry := requireJSONObject(t, entries[0])
	handlers, ok := entry["hooks"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, handlers)
	handler := requireJSONObject(t, handlers[0])
	command, ok := handler["command"].(string)
	require.True(t, ok)
	return command
}

func flatHookCommand(t *testing.T, path, container, event string) string {
	t.Helper()
	document := readJSONDocument(t, path)
	events := requireJSONObject(t, document[container])
	entries, ok := events[event].([]any)
	require.True(t, ok)
	require.NotEmpty(t, entries)
	handler := requireJSONObject(t, entries[0])
	command, ok := handler["command"].(string)
	require.True(t, ok)
	return command
}

func readJSONDocument(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(data, &document))
	return document
}

func writeJSONDocument(t *testing.T, path string, document map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(document, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(data, '\n'), 0o644))
}

func requireJSONObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	require.True(t, ok)
	return object
}

func assertMappingContainsPath(t *testing.T, mappings []adapter.FileMapping, path string) {
	t.Helper()
	for _, mapping := range mappings {
		if filepath.ToSlash(mapping.TargetPath) == path {
			return
		}
	}
	t.Errorf("mapping does not contain %s", path)
}

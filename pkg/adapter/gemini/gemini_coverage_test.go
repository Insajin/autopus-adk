package gemini

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Extended Skills ---

func TestRenderExtendedSkills(t *testing.T) {
	t.Parallel()
	files, err := NewWithRoot(t.TempDir()).renderExtendedSkills()
	require.NoError(t, err)
	assert.NotEmpty(t, files)
	for _, f := range files {
		assert.Contains(t, f.TargetPath, ".gemini/skills/autopus/")
		assert.Equal(t, adapter.OverwriteAlways, f.OverwritePolicy)
	}
}

func TestLogTransformReport_Nil(t *testing.T) {
	t.Parallel()
	logTransformReport("gemini", nil)
}

func TestLogTransformReport_WithData(t *testing.T) {
	t.Parallel()
	report := &pkgcontent.TransformReport{
		Compatible:   []string{"a", "b"},
		Incompatible: []string{"c"},
	}
	logTransformReport("gemini", report)
}

// --- Settings: toStringSlice ---

func TestToStringSlice_NilInput(t *testing.T) {
	t.Parallel()
	assert.Nil(t, toStringSlice(nil))
}

func TestToStringSlice_NonArray(t *testing.T) {
	t.Parallel()
	assert.Nil(t, toStringSlice("not-an-array"))
}

func TestToStringSlice_MixedTypes(t *testing.T) {
	t.Parallel()
	result := toStringSlice([]any{"hello", 42, "world", true})
	assert.Equal(t, []string{"hello", "world"}, result)
}

func TestToStringSlice_AllStrings(t *testing.T) {
	t.Parallel()
	result := toStringSlice([]any{"a", "b", "c"})
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestToStringSlice_EmptyArray(t *testing.T) {
	t.Parallel()
	assert.Empty(t, toStringSlice([]any{}))
}

// --- Settings: mergeUnique ---

func TestMergeUnique_NoDuplicates(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"a", "b", "c", "d"}, mergeUnique([]string{"a", "b"}, []string{"c", "d"}))
}

func TestMergeUnique_WithDuplicates(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"a", "b", "c"}, mergeUnique([]string{"a", "b"}, []string{"b", "c"}))
}

func TestMergeUnique_EmptyBase(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"a", "b"}, mergeUnique(nil, []string{"a", "b"}))
}

// --- Settings: mergeSettingsMaps ---

func TestMergeSettingsMaps_DeepMerge(t *testing.T) {
	t.Parallel()
	existing := map[string]any{
		"hooks": map[string]any{"userKey": "userVal"},
		"theme": "dark",
	}
	newSettings := map[string]any{
		"hooks":   map[string]any{"autopusKey": "autopusVal"},
		"version": "1.0",
	}
	result := mergeSettingsMaps(existing, newSettings)
	hooks := result["hooks"].(map[string]any)
	assert.Equal(t, "userVal", hooks["userKey"])
	assert.Equal(t, "autopusVal", hooks["autopusKey"])
	assert.Equal(t, "dark", result["theme"])
}

// --- InstallHooks ---

func TestInstallHooks_WithHooksAndPerms(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hooks := []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: ".*", Type: "command", Command: "echo pre", Timeout: 10},
		{Event: "PostToolUse", Matcher: ".*", Type: "command", Command: "echo post", Timeout: 5},
	}
	perms := &adapter.PermissionSet{Allow: []string{"Bash", "Read"}, Deny: []string{"Write"}}
	require.NoError(t, NewWithRoot(dir).InstallHooks(context.Background(), hooks, perms))
	data, _ := os.ReadFile(filepath.Join(dir, ".gemini", "settings.json"))
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.Contains(t, settings["hooks"].(map[string]any), "PreToolUse")
	assert.Len(t, settings["permissions"].(map[string]any)["allow"].([]any), 2)
}

func TestInstallHooks_MergesExistingPerms(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".gemini")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	d, _ := json.Marshal(map[string]any{"permissions": map[string]any{"allow": []any{"Existing"}}})
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), d, 0644))
	require.NoError(t, NewWithRoot(dir).InstallHooks(context.Background(), nil,
		&adapter.PermissionSet{Allow: []string{"Bash"}, Deny: []string{"Write"}}))
	updated, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	var settings map[string]any
	require.NoError(t, json.Unmarshal(updated, &settings))
	assert.Len(t, settings["permissions"].(map[string]any)["allow"].([]any), 2)
}

func TestInstallHooks_PreservesNonManagedEvents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".gemini")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	d, _ := json.Marshal(map[string]any{"hooks": map[string]any{"Custom": []any{map[string]any{"command": "u.sh"}}}})
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), d, 0644))
	hooks := []adapter.HookConfig{{Event: "PreToolUse", Matcher: ".*", Type: "command", Command: "echo", Timeout: 5}}
	require.NoError(t, NewWithRoot(dir).InstallHooks(context.Background(), hooks, nil))
	updated, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	var settings map[string]any
	require.NoError(t, json.Unmarshal(updated, &settings))
	hooksMap := settings["hooks"].(map[string]any)
	assert.Contains(t, hooksMap, "Custom")
	assert.Contains(t, hooksMap, "PreToolUse")
}

func TestInstallHooks_InvalidExistingJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".gemini")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte("{broken"), 0644))
	hooks := []adapter.HookConfig{{Event: "PreToolUse", Matcher: ".*", Type: "command", Command: "echo", Timeout: 5}}
	require.NoError(t, NewWithRoot(dir).InstallHooks(context.Background(), hooks, nil))
}

// --- Lifecycle ---

func TestValidate_NoMarkerSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "GEMINI.md"), []byte("# No Marker\n"), 0644))

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)
	found := false
	for _, e := range errs {
		if e.Level == "warning" && e.File == "GEMINI.md" {
			found = true
		}
	}
	assert.True(t, found, "should warn about missing marker")
}

func TestValidate_MarkerPresentButMissingSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	content := markerBegin + "\ncontent\n" + markerEnd + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "GEMINI.md"), []byte(content), 0644))

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)
	hasSkillErr := false
	for _, e := range errs {
		if e.Level == "error" && strings.Contains(e.Message, "SKILL.md") {
			hasSkillErr = true
		}
	}
	assert.True(t, hasSkillErr, "should report missing skill files")
}

// --- prepareFiles ---

func TestPrepareFiles_HasAllCategories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.prepareFiles(cfg)
	require.NoError(t, err)

	hasGeminiMD, hasSkills, hasRules, hasCommands, hasSettings := false, false, false, false, false
	for _, f := range files {
		switch {
		case f.TargetPath == "GEMINI.md":
			hasGeminiMD = true
		case strings.Contains(f.TargetPath, "skills"):
			hasSkills = true
		case strings.Contains(f.TargetPath, "rules"):
			hasRules = true
		case strings.Contains(f.TargetPath, "commands"):
			hasCommands = true
		case strings.Contains(f.TargetPath, "settings"):
			hasSettings = true
		}
	}
	assert.True(t, hasGeminiMD && hasSkills && hasRules && hasCommands && hasSettings)
}

// --- Render/prepare helpers ---

func TestPrepareRuleMappings(t *testing.T) {
	t.Parallel()
	files, err := NewWithRoot(t.TempDir()).prepareRuleMappings(config.DefaultFullConfig("test"))
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

func TestPrepareCommandMappings(t *testing.T) {
	t.Parallel()
	files, err := NewWithRoot(t.TempDir()).prepareCommandMappings(config.DefaultFullConfig("test"))
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

func TestPrepareSkillMappings(t *testing.T) {
	t.Parallel()
	files, err := NewWithRoot(t.TempDir()).prepareSkillMappings(config.DefaultFullConfig("test"))
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

func TestGenerateSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	files, err := a.generateSettings(cfg)
	require.NoError(t, err)
	require.Len(t, files, 1)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(files[0].Content, &parsed))
}

func TestGenerateSettings_MergesExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	settingsDir := filepath.Join(dir, ".gemini")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	data, _ := json.Marshal(map[string]any{"userKey": "userVal"})
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644))
	files, err := a.generateSettings(cfg)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(files[0].Content, &parsed))
	assert.Equal(t, "userVal", parsed["userKey"])
}

func TestGenerateSettings_InvalidExistingJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	settingsDir := filepath.Join(dir, ".gemini")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte("{bad"), 0644))
	files, err := a.generateSettings(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

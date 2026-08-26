package claude

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

type claudeSkillFrontmatter struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Extra       map[string]any `yaml:",inline"`
}

type claudeAgentFrontmatter struct {
	Name   string   `yaml:"name"`
	Model  string   `yaml:"model"`
	Effort string   `yaml:"effort"`
	Skills []string `yaml:"skills"`
}

func decodeClaudeFrontmatter(t *testing.T, content []byte, out any) {
	t.Helper()
	parts := strings.SplitN(string(content), "---", 3)
	require.Len(t, parts, 3)
	require.NoError(t, yaml.Unmarshal([]byte(parts[1]), out))
}

func TestPrepareFiles_ClaudeSkillsUseNativeLayoutAndResolveAgentReferences(t *testing.T) {
	t.Parallel()
	files, err := NewWithRoot(t.TempDir()).prepareFiles(config.DefaultFullConfig("native-skills"))
	require.NoError(t, err)

	skills := make(map[string]bool)
	for _, file := range files {
		path := filepath.ToSlash(file.TargetPath)
		if !strings.HasPrefix(path, ".claude/skills/") {
			continue
		}
		assert.Equal(t, "SKILL.md", filepath.Base(path), path)
		var meta claudeSkillFrontmatter
		decodeClaudeFrontmatter(t, file.Content, &meta)
		dirName := filepath.Base(filepath.Dir(path))
		assert.Equal(t, dirName, meta.Name, path)
		if meta.Description == "" {
			t.Errorf("%s has empty skill description", path)
		}
		assert.Empty(t, meta.Extra, "Claude skill frontmatter must contain documented fields only: %s", path)
		skills[meta.Name] = true
	}

	for _, file := range files {
		path := filepath.ToSlash(file.TargetPath)
		if !strings.HasPrefix(path, ".claude/agents/") {
			continue
		}
		var meta claudeAgentFrontmatter
		decodeClaudeFrontmatter(t, file.Content, &meta)
		for _, name := range meta.Skills {
			assert.True(t, skills[name], "%s references unresolved skill %q", path, name)
		}
	}
}

func TestPrepareFiles_ClaudeAgentQualityProjectsModelAndEffortTogether(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		mode       string
		agent      string
		wantModel  string
		wantEffort string
	}{
		{name: "balanced standard", mode: "balanced", agent: "tester", wantModel: "sonnet", wantEffort: "medium"},
		{name: "ultra premium", mode: "ultra", agent: "executor", wantModel: "opus", wantEffort: "max"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DefaultFullConfig("quality")
			cfg.Quality.Providers = map[string]string{config.QualityProviderClaude: test.mode}
			files, err := NewWithRoot(t.TempDir()).prepareFiles(cfg)
			require.NoError(t, err)

			var got claudeAgentFrontmatter
			for _, file := range files {
				if filepath.Base(file.TargetPath) == test.agent+".md" && strings.Contains(filepath.ToSlash(file.TargetPath), "/agents/") {
					decodeClaudeFrontmatter(t, file.Content, &got)
					break
				}
			}
			assert.Equal(t, test.wantModel, got.Model)
			assert.Equal(t, test.wantEffort, got.Effort)
		})
	}
}

func TestInstallHooks_PrunesRemovedTeamLifecyclePermissions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	settingsDir := filepath.Join(root, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(settingsDir, "settings.json"),
		[]byte(`{"permissions":{"allow":["TeamCreate","TeamDelete","UserTool"]}}`),
		0o644,
	))

	err := NewWithRoot(root).InstallHooks(context.Background(), nil, &adapter.PermissionSet{
		Allow: []string{"Agent", "SendMessage"},
	})

	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	permissions, ok := settings["permissions"].(map[string]any)
	require.True(t, ok)
	allow, ok := permissions["allow"].([]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []any{"UserTool", "Agent", "SendMessage"}, allow)
}

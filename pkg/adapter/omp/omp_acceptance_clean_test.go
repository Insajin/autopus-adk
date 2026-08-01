package omp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/config"
)

// backupCopies returns every backed-up path (relative to the backup root) found
// under .autopus/backup/.
func backupCopies(t *testing.T, root string) map[string]bool {
	t.Helper()
	base := filepath.Join(root, ".autopus", "backup")
	found := make(map[string]bool)
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return relErr
		}
		// Strip the timestamped backup session directory.
		parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
		if len(parts) == 2 {
			found[parts[1]] = true
			return nil
		}
		found[filepath.ToSlash(rel)] = true
		return nil
	})
	require.NoError(t, err)
	return found
}

// TestOMPAcceptance_S13_UserOwnedOMPSurfacePreserved covers REQ-017.
func TestOMPAcceptance_S13_UserOwnedOMPSurfacePreserved(t *testing.T) {
	dir := generateOMPOnly(t)

	userRule := filepath.Join(dir, ".omp", "rules", "my-rule.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(userRule), 0o755))
	require.NoError(t, os.WriteFile(userRule, []byte("# user rule\n"), 0o644))
	userRulesMD := filepath.Join(dir, ".omp", "RULES.md")
	require.NoError(t, os.WriteFile(userRulesMD, []byte("# sticky rules\n"), 0o644))

	// User keys added outside the managed marker section.
	cfgPath := filepath.Join(dir, configFile)
	original, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath,
		append([]byte("disabledProviders:\n  - anthropic\n\n"), original...), 0o644))

	// User edits one managed agent; another stays untouched.
	editedAgent := filepath.Join(dir, ".omp", "agents", "executor.md")
	require.NoError(t, os.WriteFile(editedAgent, []byte("# hand edited\n"), 0o644))
	untouchedAgent := filepath.Join(dir, ".omp", "agents", "planner.md")
	require.FileExists(t, untouchedAgent)

	// `auto platform remove omp` drops omp from the platform list before Clean.
	remaining := config.DefaultFullConfig("omp-acceptance")
	remaining.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(dir, remaining))

	require.NoError(t, NewWithRoot(dir).Clean(context.Background()))

	assert.FileExists(t, userRule, ".omp/rules/my-rule.md must survive")
	assert.FileExists(t, userRulesMD, ".omp/RULES.md must survive")
	assert.DirExists(t, filepath.Join(dir, ".omp"), ".omp/ must not be removed wholesale")

	backups := backupCopies(t, dir)
	assert.True(t, backups[".omp/agents/executor.md"],
		"a user-edited managed file must be backed up before removal, found backups: %v", backups)
	assert.False(t, backups[".omp/agents/planner.md"],
		"an unmodified managed file must be removed without a backup")

	assert.NoFileExists(t, editedAgent)
	assert.NoFileExists(t, untouchedAgent)
}

// TestOMPAcceptance_E3_ConfigUserKeysSurviveUpdate covers REQ-007.
func TestOMPAcceptance_E3_ConfigUserKeysSurviveUpdate(t *testing.T) {
	dir := generateOMPOnly(t)
	cfgPath := filepath.Join(dir, configFile)

	original, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath,
		append([]byte("disabledProviders:\n  - anthropic\n\n"), original...), 0o644))

	cfg := config.DefaultFullConfig("omp-acceptance")
	cfg.Platforms = []string{"omp"}
	_, err = NewWithRoot(dir).Update(context.Background(), cfg)
	require.NoError(t, err)

	updated, err := os.ReadFile(cfgPath)
	require.NoError(t, err)

	var parsed struct {
		DisabledProviders []string `yaml:"disabledProviders"`
		Skills            struct {
			CustomDirectories []string `yaml:"customDirectories"`
		} `yaml:"skills"`
	}
	require.NoError(t, yaml.Unmarshal(updated, &parsed),
		"the merged config must still parse as YAML")
	assert.Equal(t, []string{"anthropic"}, parsed.DisabledProviders,
		"user keys outside the marker section must survive regeneration")
	assert.Equal(t, []string{".agents/skills"}, parsed.Skills.CustomDirectories)
	assert.Equal(t, 1, strings.Count(string(updated), markerBeginYml),
		"regeneration must not duplicate the managed section")
}

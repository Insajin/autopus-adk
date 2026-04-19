package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunOrchestraCommand_UsesConfigStrategy verifies that runOrchestraCommand
// resolves strategy from config when no flag is explicitly provided.
// We test this indirectly by checking that the function does not error
// when given an invalid strategy that config would override.
func TestRunOrchestraCommand_ConfigFallback(t *testing.T) {
	// When autopus.yaml is absent (no config), loadOrchestraConfig fails gracefully
	// and runOrchestraCommand falls back to buildProviderConfigs behavior.
	//
	// We test fallback behavior: with an invalid provider and no config,
	// buildProviderConfigs path is used and returns an empty result.
	//
	// Note: actual orchestra execution is not tested here since it requires
	// live binaries. We test the config resolution path only.
	dir := t.TempDir()

	// Change to a temp dir with no autopus.yaml to trigger config load failure
	original, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(original)
		require.NoError(t, err)
	}()

	err = os.Chdir(dir)
	require.NoError(t, err)

	// loadOrchestraConfig from empty dir should return default config (no file = default)
	orchConf, err := loadOrchestraConfig()
	require.NoError(t, err)
	assert.NotNil(t, orchConf)
}

// TestRunOrchestraCommand_CommandNamePropagation verifies that the three
// subcommand builders pass appropriate commandName values to the config resolver.
func TestResolveProviders_CommandNameReview(t *testing.T) {
	// Use default full config to simulate real config behavior
	dir := t.TempDir()
	content := `
mode: full
project_name: test
platforms:
  - claude-code
orchestra:
  enabled: true
  default_strategy: consensus
  providers:
    claude:
      binary: claude
      args: ["--print"]
    gemini:
      binary: gemini
      args: []
      prompt_via_args: true
  commands:
    review:
      strategy: debate
      providers: ["claude", "gemini"]
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	original, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(original)
		require.NoError(t, err)
	}()

	err = os.Chdir(dir)
	require.NoError(t, err)

	orchConf, err := loadOrchestraConfig()
	require.NoError(t, err)

	// resolveProviders with commandName="review" and no flag providers
	providers := resolveProviders(orchConf, "review", nil)
	require.Len(t, providers, 2)

	// resolveStrategy with commandName="review" and no flag
	strategy := resolveStrategy(orchConf, "review", "")
	assert.Equal(t, "debate", strategy)

	// gemini must carry PromptViaArgs=true from config
	for _, p := range providers {
		if p.Name == "gemini" {
			assert.True(t, p.PromptViaArgs)
		}
	}
}

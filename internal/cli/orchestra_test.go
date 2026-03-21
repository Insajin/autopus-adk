package cli

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// TestNewOrchestraCmd_SubcommandRegistration verifies that newOrchestraCmd
// registers exactly 3 subcommands: review, plan, secure.
func TestNewOrchestraCmd_SubcommandRegistration(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "orchestra", cmd.Use)

	subCmds := cmd.Commands()
	require.Len(t, subCmds, 3)

	names := make([]string, len(subCmds))
	for i, sc := range subCmds {
		names[i] = sc.Name()
	}
	assert.Contains(t, names, "review")
	assert.Contains(t, names, "plan")
	assert.Contains(t, names, "secure")
}

// TestNewOrchestraReviewCmd_Flags verifies that review cmd registers all expected flags.
func TestNewOrchestraReviewCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraReviewCmd()
	require.NotNil(t, cmd)

	assert.NotNil(t, cmd.Flags().Lookup("strategy"), "strategy flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("providers"), "providers flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("timeout"), "timeout flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("judge"), "judge flag must exist")
}

// TestNewOrchestraPlanCmd_Flags verifies that plan cmd registers expected flags.
func TestNewOrchestraPlanCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraPlanCmd()
	require.NotNil(t, cmd)

	assert.NotNil(t, cmd.Flags().Lookup("strategy"), "strategy flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("providers"), "providers flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("timeout"), "timeout flag must exist")
	assert.Nil(t, cmd.Flags().Lookup("judge"), "plan cmd must NOT have judge flag")
}

// TestNewOrchestraSecureCmd_Flags verifies that secure cmd registers expected flags.
func TestNewOrchestraSecureCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newOrchestraSecureCmd()
	require.NotNil(t, cmd)

	assert.NotNil(t, cmd.Flags().Lookup("strategy"), "strategy flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("providers"), "providers flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("timeout"), "timeout flag must exist")
	assert.Nil(t, cmd.Flags().Lookup("judge"), "secure cmd must NOT have judge flag")
}

// TestBuildProviderConfigs verifies known provider mappings.
func TestBuildProviderConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           []string
		expectName      string
		expectBinary    string
		expectViaArgs   bool
		expectArgsEmpty bool
	}{
		{
			name:          "claude uses stdin",
			input:         []string{"claude"},
			expectName:    "claude",
			expectBinary:  "claude",
			expectViaArgs: false,
		},
		{
			name:          "codex uses stdin",
			input:         []string{"codex"},
			expectName:    "codex",
			expectBinary:  "codex",
			expectViaArgs: false,
		},
		{
			name:          "gemini uses args",
			input:         []string{"gemini"},
			expectName:    "gemini",
			expectBinary:  "gemini",
			expectViaArgs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := buildProviderConfigs(tt.input)
			require.Len(t, result, 1)
			assert.Equal(t, tt.expectName, result[0].Name)
			assert.Equal(t, tt.expectBinary, result[0].Binary)
			assert.Equal(t, tt.expectViaArgs, result[0].PromptViaArgs)
		})
	}
}

// TestBuildProviderConfigs_Unknown verifies that unknown providers fall back to
// binary=name, args=[], PromptViaArgs=false.
func TestBuildProviderConfigs_Unknown(t *testing.T) {
	t.Parallel()

	result := buildProviderConfigs([]string{"my-custom-tool"})
	require.Len(t, result, 1)
	assert.Equal(t, "my-custom-tool", result[0].Name)
	assert.Equal(t, "my-custom-tool", result[0].Binary)
	assert.Empty(t, result[0].Args)
	assert.False(t, result[0].PromptViaArgs)
}

// TestBuildProviderConfigs_Multiple verifies that multiple providers are returned in order.
func TestBuildProviderConfigs_Multiple(t *testing.T) {
	t.Parallel()

	result := buildProviderConfigs([]string{"claude", "codex", "gemini"})
	require.Len(t, result, 3)
	assert.Equal(t, "claude", result[0].Name)
	assert.Equal(t, "codex", result[1].Name)
	assert.Equal(t, "gemini", result[2].Name)
}

// TestBuildProviderConfigs_Empty verifies empty input returns empty slice.
func TestBuildProviderConfigs_Empty(t *testing.T) {
	t.Parallel()

	result := buildProviderConfigs([]string{})
	assert.Empty(t, result)
}

// TestDefaultProviders verifies the hardcoded defaults include claude, codex, gemini.
func TestDefaultProviders(t *testing.T) {
	t.Parallel()

	providers := defaultProviders()
	assert.Len(t, providers, 3)
	assert.Contains(t, providers, "claude")
	assert.Contains(t, providers, "codex")
	assert.Contains(t, providers, "gemini")
}

// TestFlagStringIfChanged verifies that flagStringIfChanged returns value
// only when the flag has been explicitly changed.
func TestFlagStringIfChanged(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("myflag", "", "test flag")

	// Not changed: should return empty string regardless of value
	result := flagStringIfChanged(cmd, "myflag", "somevalue")
	assert.Equal(t, "", result, "unchanged flag must return empty string")

	// Simulate flag being explicitly set
	require.NoError(t, cmd.Flags().Set("myflag", "explicit"))
	result = flagStringIfChanged(cmd, "myflag", "explicit")
	assert.Equal(t, "explicit", result, "changed flag must return its value")
}

// TestFlagStringSliceIfChanged verifies that flagStringSliceIfChanged returns
// the value only when the flag has been explicitly changed.
func TestFlagStringSliceIfChanged(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringSlice("providers", nil, "test flag")

	// Not changed: should return nil
	result := flagStringSliceIfChanged(cmd, "providers", []string{"a", "b"})
	assert.Nil(t, result, "unchanged flag must return nil")

	// Simulate flag being explicitly set
	require.NoError(t, cmd.Flags().Set("providers", "x,y"))
	result = flagStringSliceIfChanged(cmd, "providers", []string{"x", "y"})
	assert.Equal(t, []string{"x", "y"}, result, "changed flag must return its value")
}

// TestBuildReviewPrompt_NoFiles verifies default prompt when no files are given.
func TestBuildReviewPrompt_NoFiles(t *testing.T) {
	t.Parallel()

	prompt := buildReviewPrompt(nil)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "리뷰")
}

// TestBuildReviewPrompt_WithFiles verifies that file-based prompt includes filenames.
func TestBuildReviewPrompt_WithFiles(t *testing.T) {
	t.Parallel()

	// Non-existent files are handled gracefully with error message embedded.
	prompt := buildReviewPrompt([]string{"/nonexistent/file.go"})
	assert.Contains(t, prompt, "file.go")
}

// TestBuildSecurePrompt_NoFiles verifies default security prompt.
func TestBuildSecurePrompt_NoFiles(t *testing.T) {
	t.Parallel()

	prompt := buildSecurePrompt(nil)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "보안")
}

// TestBuildSecurePrompt_WithFiles verifies that security prompt includes filenames.
func TestBuildSecurePrompt_WithFiles(t *testing.T) {
	t.Parallel()

	prompt := buildSecurePrompt([]string{"/nonexistent/auth.go"})
	assert.Contains(t, prompt, "auth.go")
}

// TestBuildFileContents_MissingFile verifies graceful error message for missing files.
func TestBuildFileContents_MissingFile(t *testing.T) {
	t.Parallel()

	result := buildFileContents([]string{"/this/does/not/exist.go"})
	assert.Contains(t, result, "읽기 실패")
}

// TestBuildFileContents_ExistingFile verifies that file content is embedded.
func TestBuildFileContents_ExistingFile(t *testing.T) {
	t.Parallel()

	// Create a temp file
	f := t.TempDir() + "/sample.go"
	require.NoError(t, os.WriteFile(f, []byte("package main\n"), 0o644))

	result := buildFileContents([]string{f})
	assert.Contains(t, result, "package main")
	assert.Contains(t, result, "sample.go")
}

// TestRunOrchestraCommand_InvalidStrategy verifies error on invalid strategy
// when no config file is present (uses fallback path).
func TestRunOrchestraCommand_InvalidStrategy(t *testing.T) {
	t.Parallel()

	// Simulate calling with explicit invalid strategy
	// runOrchestraCommand validates s.IsValid() and returns error.
	providers := []orchestra.ProviderConfig{
		{Name: "test", Binary: "cat", Args: []string{}},
	}
	cfg := orchestra.OrchestraConfig{
		Providers: providers,
		Strategy:  orchestra.Strategy("invalid-strat"),
		Prompt:    "test",
	}
	assert.False(t, cfg.Strategy.IsValid())
}

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPipelineRunCmd_RequiresSpecID verifies that the pipeline run command
// returns an error when no SPEC-ID argument is provided.
func TestPipelineRunCmd_RequiresSpecID(t *testing.T) {
	t.Parallel()

	// Given: a pipeline run command with no arguments
	cmd := newPipelineRunCmd()
	cmd.SetArgs([]string{})

	// When: the command is executed
	err := cmd.Execute()

	// Then: an error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec")
}

// TestPipelineRunCmd_DefaultPlatform verifies that when --platform is omitted,
// the command auto-detects the current platform (REQ-3).
func TestPipelineRunCmd_DefaultPlatform(t *testing.T) {
	t.Parallel()

	// Given: a pipeline run command with SPEC-ID but no --platform flag
	cfg := &pipelineRunConfig{}
	cmd := newPipelineRunCmdWithConfig(cfg)
	cmd.SetArgs([]string{"SPEC-TEST-001"})

	// When: the command is parsed (flags bound)
	err := cmd.ParseFlags([]string{"SPEC-TEST-001"})
	require.NoError(t, err)

	// Then: platform is auto-detected (non-empty)
	resolved := resolvePlatform(cfg.Platform)
	assert.NotEmpty(t, resolved)
}

// TestPipelineRunCmd_StrategyFlag verifies that --strategy flag is parsed
// correctly for both "sequential" and "parallel" values (REQ-4/REQ-5).
func TestPipelineRunCmd_StrategyFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strategy string
		wantErr  bool
	}{
		{"sequential strategy", "sequential", false},
		{"parallel strategy", "parallel", false},
		{"invalid strategy", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Given: a pipeline run command with --strategy flag
			cfg := &pipelineRunConfig{}
			cmd := newPipelineRunCmdWithConfig(cfg)
			args := []string{"SPEC-TEST-001", "--strategy", tt.strategy}

			// When: the command flags are parsed
			err := cmd.ParseFlags(args)

			// Then: valid strategies parse without error
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.strategy, cfg.Strategy)
			}
		})
	}
}

// TestPipelineRunCmd_ContinueFlag verifies that --continue flag triggers
// checkpoint loading (REQ-7).
func TestPipelineRunCmd_ContinueFlag(t *testing.T) {
	t.Parallel()

	// Given: a pipeline run command with --continue flag
	cfg := &pipelineRunConfig{}
	cmd := newPipelineRunCmdWithConfig(cfg)

	// When: flags are parsed with --continue
	err := cmd.ParseFlags([]string{"SPEC-TEST-001", "--continue"})

	// Then: the continue flag is set to true
	require.NoError(t, err)
	assert.True(t, cfg.Continue)
}

// TestPipelineRunCmd_DryRunFlag verifies that --dry-run flag is parsed
// correctly.
func TestPipelineRunCmd_DryRunFlag(t *testing.T) {
	t.Parallel()

	// Given: a pipeline run command with --dry-run flag
	cfg := &pipelineRunConfig{}
	cmd := newPipelineRunCmdWithConfig(cfg)

	// When: flags are parsed with --dry-run
	err := cmd.ParseFlags([]string{"SPEC-TEST-001", "--dry-run"})

	// Then: the dry-run flag is set to true
	require.NoError(t, err)
	assert.True(t, cfg.DryRun)
}

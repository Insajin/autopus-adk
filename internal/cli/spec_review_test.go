package cli

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSpecReviewCmd_Structure verifies that the spec review command
// has the correct Use field and expected flags.
func TestNewSpecReviewCmd_Structure(t *testing.T) {
	t.Parallel()

	cmd := newSpecReviewCmd()
	require.NotNil(t, cmd)

	// Use should start with "review"
	assert.Equal(t, "review <SPEC-ID>", cmd.Use)

	// strategy and timeout flags must be present
	assert.NotNil(t, cmd.Flags().Lookup("strategy"), "strategy flag must exist")
	assert.NotNil(t, cmd.Flags().Lookup("timeout"), "timeout flag must exist")
}

// TestNewSpecReviewCmd_RequiresExactlyOneArg verifies cobra arg validation.
func TestNewSpecReviewCmd_RequiresExactlyOneArg(t *testing.T) {
	t.Parallel()

	cmd := newSpecReviewCmd()
	require.NotNil(t, cmd)

	// Args constraint is cobra.ExactArgs(1)
	err := cmd.Args(cmd, []string{})
	assert.Error(t, err, "no args should fail validation")

	err = cmd.Args(cmd, []string{"SPEC-001"})
	assert.NoError(t, err, "exactly one arg should pass")

	err = cmd.Args(cmd, []string{"SPEC-001", "extra"})
	assert.Error(t, err, "two args should fail validation")
}

// TestBuildReviewProviders_NoNames verifies that empty names list returns empty slice.
func TestBuildReviewProviders_NoNames(t *testing.T) {
	t.Parallel()

	result := buildReviewProviders([]string{})
	assert.Empty(t, result, "empty input should return empty slice")
}

// TestBuildReviewProviders_SkipsMissingBinaries verifies that binaries not on PATH
// are silently skipped.
func TestBuildReviewProviders_SkipsMissingBinaries(t *testing.T) {
	t.Parallel()

	// Use a binary name guaranteed not to exist
	result := buildReviewProviders([]string{"binary_that_does_not_exist_xyz_autopus_test"})
	assert.Empty(t, result, "missing binary should be skipped")
}

// TestBuildReviewProviders_SkipsMultipleMissingBinaries verifies batch skip behavior.
func TestBuildReviewProviders_SkipsMultipleMissingBinaries(t *testing.T) {
	t.Parallel()

	result := buildReviewProviders([]string{
		"no_such_binary_aaa",
		"no_such_binary_bbb",
		"no_such_binary_ccc",
	})
	assert.Empty(t, result, "all missing binaries should be skipped")
}

// TestRunSpecReview_SpecLoadError verifies that runSpecReview returns an error
// when the SPEC directory does not exist.
func TestRunSpecReview_SpecLoadError(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := t.TempDir()

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	execErr := runSpecReview(context.Background(), "SPEC-NONEXISTENT-999", "", 0)
	assert.Error(t, execErr, "runSpecReview should fail when SPEC dir is missing")
}

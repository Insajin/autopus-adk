package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
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

func TestResolveSpecReviewProviderNames_Default(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("test-project")
	cfg.Spec.ReviewGate.Providers = []string{"claude", "gemini"}

	result := resolveSpecReviewProviderNames(cfg, false)
	assert.Equal(t, []string{"claude", "gemini"}, result)
}

func TestResolveSpecReviewProviderNames_MultiExpandsAndDeduplicates(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("test-project")
	cfg.Spec.ReviewGate.Providers = []string{"claude"}
	cfg.Orchestra.Commands["review"] = config.CommandEntry{
		Strategy:  "debate",
		Providers: []string{"claude", "codex"},
	}
	cfg.Orchestra.Providers = map[string]config.ProviderEntry{
		"gemini": {Binary: "gemini"},
		"codex":  {Binary: "codex"},
		"claude": {Binary: "claude"},
	}

	result := resolveSpecReviewProviderNames(cfg, true)
	assert.Equal(t, []string{"claude", "codex", "gemini"}, result)
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

func TestRunSpecReview_PassApprovesSpec(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-REVIEW-001")
	setFakeProviderOnPath(t, dir, "claude")

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{{Provider: "claude", Output: "VERDICT: PASS"}}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	require.NoError(t, runSpecReview(context.Background(), "SPEC-REVIEW-001", "consensus", 10))

	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	assert.Equal(t, "approved", doc.Status)
}

func TestRunSpecReview_ReviseDoesNotApproveSpec(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-REVIEW-002")
	setFakeProviderOnPath(t, dir, "claude")

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	callCount := 0
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		callCount++
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{{
			Provider: "claude",
			Output:   "VERDICT: REVISE\nFINDING: [major] [correctness] REQ-001 Needs revision",
		}}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	require.NoError(t, runSpecReview(context.Background(), "SPEC-REVIEW-002", "consensus", 10))

	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	assert.Equal(t, "draft", doc.Status)
	assert.GreaterOrEqual(t, callCount, 1)
}

func scaffoldReviewSpec(t *testing.T, dir, specID string) string {
	t.Helper()
	require.NoError(t, spec.Scaffold(dir, specID[len("SPEC-"):], "리뷰 테스트"))
	return filepath.Join(dir, ".autopus", "specs", specID)
}

func setFakeProviderOnPath(t *testing.T, dir, binary string) {
	t.Helper()
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	path := filepath.Join(binDir, binary)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

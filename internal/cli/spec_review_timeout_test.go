package cli

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func TestRunSpecReview_UsesOrchestraTimeoutFromConfig(t *testing.T) {
	dir := t.TempDir()
	scaffoldReviewSpec(t, dir, "SPEC-REVIEW-TIMEOUT-001")
	setFakeProviderOnPath(t, dir, "claude")

	cfg := config.DefaultFullConfig("test-project")
	cfg.Spec.ReviewGate.Providers = []string{"claude"}
	cfg.Orchestra.TimeoutSeconds = 240
	require.NoError(t, config.Save(dir, cfg))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	var capturedTimeout int
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		capturedTimeout = cfg.TimeoutSeconds
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{{Provider: "claude", Output: "VERDICT: PASS"}}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	require.NoError(t, runSpecReview(context.Background(), "SPEC-REVIEW-TIMEOUT-001", "consensus", 0))
	assert.Equal(t, 240, capturedTimeout)
}

func TestRunSpecReview_CLITimeoutOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	scaffoldReviewSpec(t, dir, "SPEC-REVIEW-TIMEOUT-FLAG-001")
	setFakeProviderOnPath(t, dir, "claude")

	cfg := config.DefaultFullConfig("test-project")
	cfg.Spec.ReviewGate.Providers = []string{"claude"}
	cfg.Orchestra.TimeoutSeconds = 240
	require.NoError(t, config.Save(dir, cfg))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	var capturedTimeout int
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		capturedTimeout = cfg.TimeoutSeconds
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{{Provider: "claude", Output: "VERDICT: PASS"}}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	require.NoError(t, runSpecReview(context.Background(), "SPEC-REVIEW-TIMEOUT-FLAG-001", "consensus", 90))
	assert.Equal(t, 90, capturedTimeout)
}

func TestRunSpecReview_AppliesOrchestraMigrationToClaudeProvider(t *testing.T) {
	dir := t.TempDir()
	scaffoldReviewSpec(t, dir, "SPEC-REVIEW-MIGRATE-001")
	setFakeProviderOnPath(t, dir, "claude")

	cfg := config.DefaultFullConfig("test-project")
	cfg.Spec.ReviewGate.Providers = []string{"claude"}
	cfg.Orchestra.Providers["claude"] = config.ProviderEntry{
		Binary:   "claude",
		Args:     []string{"--print", "--model", "opus", "--effort", "max"},
		PaneArgs: []string{"-p", "--model", "opus", "--effort", "max"},
	}
	require.NoError(t, config.Save(dir, cfg))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	var capturedProvider orchestra.ProviderConfig
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		require.Len(t, cfg.Providers, 1)
		capturedProvider = cfg.Providers[0]
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{{Provider: "claude", Output: "VERDICT: PASS"}}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	require.NoError(t, runSpecReview(context.Background(), "SPEC-REVIEW-MIGRATE-001", "consensus", 0))
	assert.Equal(t, []string{"--print", "--model", "opus", "--effort", "high"}, capturedProvider.Args)
	assert.Equal(t, 480*time.Second, capturedProvider.ExecutionTimeout)
}

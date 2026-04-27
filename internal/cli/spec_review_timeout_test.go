package cli

import (
	"context"
	"os"
	"testing"

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

package cli

import (
	"context"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

var (
	specReviewRunOrchestra   = orchestra.RunOrchestra
	specReviewBuildProviders = buildReviewProviders
)

func syncReviewedSpecStatus(specDir string, result *spec.ReviewResult) error {
	if result == nil {
		return nil
	}
	if result.Verdict != spec.VerdictPass || hasActiveFindings(result.Findings) {
		return nil
	}
	return spec.UpdateStatus(specDir, "approved")
}

type specReviewRunner func(context.Context, orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error)

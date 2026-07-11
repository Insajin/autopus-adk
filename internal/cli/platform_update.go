package cli

import (
	"context"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

func updateHarnessPlatform(
	ctx context.Context,
	dir string,
	platform string,
	cfg *config.HarnessConfig,
) (bool, error) {
	switch platform {
	case "claude-code":
		_, err := claude.NewWithRoot(dir).Update(ctx, cfg)
		return true, err
	case "codex":
		_, err := codex.NewWithRoot(dir).Update(ctx, cfg)
		return true, err
	case "antigravity-cli":
		_, err := gemini.NewWithRoot(dir).Update(ctx, cfg)
		return true, err
	case "opencode":
		_, err := opencode.NewWithRoot(dir).Update(ctx, cfg)
		return true, err
	default:
		return false, nil
	}
}

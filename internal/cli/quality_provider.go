package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
)

const qualityProviderInherit = "inherit"

func newQualityProviderCmd(apply *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "provider <claude|codex> <preset|inherit>",
		Short: "Set or clear a provider-specific quality override",
		Long: "Set a provider-specific quality mode while keeping quality.default as the fallback. " +
			"`claude-code` is accepted as an alias for `claude`; `inherit` removes the override.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQualityProviderSet(cmd, args[0], args[1], apply != nil && *apply)
		},
	}
}

// @AX:WARN [AUTO]: This provider command path contains eight validation, persistence, and apply decision branches. @AX:SPEC SPEC-PROVIDER-QUALITY-001
// @AX:REASON [AUTO]: Provider normalization, inherit removal, saved effective mode, and provider-scoped apply must remain consistent.
func runQualityProviderSet(cmd *cobra.Command, rawProvider, rawPreset string, apply bool) error {
	dir, cfg, err := loadQualityConfig(cmd)
	if err != nil {
		return err
	}
	provider, ok := config.NormalizeQualityProvider(rawProvider)
	if !ok {
		return fmt.Errorf(
			"unknown quality provider %q (available: claude, codex)",
			strings.TrimSpace(rawProvider),
		)
	}

	preset := strings.TrimSpace(rawPreset)
	if preset == qualityProviderInherit {
		if err := removeQualityProvider(dir, cfg, provider); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "quality.providers.%s = inherit\n", provider)
	} else {
		if err := validateQualityChoice(cfg, preset); err != nil {
			return err
		}
		if err := saveQualityProvider(dir, cfg, provider, preset); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "quality.providers.%s = %s\n", provider, preset)
	}
	fmt.Fprintf(
		cmd.OutOrStdout(),
		"quality.effective.%s = %s\n",
		provider,
		cfg.Quality.EffectiveMode(provider),
	)

	if !apply {
		return nil
	}
	platform := qualityProviderPlatform(provider)
	if !qualityPlatformConfigured(cfg.Platforms, platform) {
		fmt.Fprintf(cmd.OutOrStdout(), "  skipped unconfigured platform: %s\n", platform)
		fmt.Fprintln(cmd.OutOrStdout(), "quality.applied_platforms = 0")
		return nil
	}
	return applyQualityHarnessPlatforms(
		cmd,
		dir,
		cfg,
		[]string{platform},
		"auto quality provider "+
			shellQuoteQualityArg(provider)+" "+
			shellQuoteQualityArg(preset)+" --apply",
	)
}

func qualityPlatformConfigured(platforms []string, target string) bool {
	for _, platform := range platforms {
		if platform == target {
			return true
		}
	}
	return false
}

func qualityProviderPlatform(provider string) string {
	if provider == config.QualityProviderClaude {
		return "claude-code"
	}
	return "codex"
}

func writeQualityProviderStatus(out io.Writer, quality config.QualityConf, provider string) {
	configured := qualityProviderInherit
	if preset := strings.TrimSpace(quality.Providers[provider]); preset != "" {
		configured = preset
	}
	fmt.Fprintf(out, "quality.providers.%s = %s\n", provider, configured)
	fmt.Fprintf(out, "quality.effective.%s = %s\n", provider, quality.EffectiveMode(provider))
}

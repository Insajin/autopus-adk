package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/design"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// buildFileContents reads each file and returns the formatted contents. A
// single unreadable entry aborts the whole batch (GitHub issue #37): silently
// embedding a "읽기 실패" marker produced prompts that paired missing content
// with the topic-isolation instruction, so providers could neither read the
// file nor fall back to disk — every run ended in "리뷰 불가".
func buildFileContents(files []string) (string, error) {
	var sb strings.Builder
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("파일 읽기 실패: %s: %w", f, err)
		}
		fmt.Fprintf(&sb, "--- %s ---\n```\n%s\n```\n\n", f, string(content))
	}
	return sb.String(), nil
}

// buildReviewPrompt builds the review prompt, including file contents if provided.
func buildReviewPrompt(files []string) (string, error) {
	if len(files) == 0 {
		return "현재 프로젝트의 코드를 리뷰해주세요. 품질, 가독성, 잠재적 버그를 중심으로 분석하세요.", nil
	}
	contents, err := buildFileContents(files)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("다음 파일들을 코드 리뷰해주세요:\n\n")
	sb.WriteString(contents)
	if section := buildReviewDesignContext(files); section != "" {
		sb.WriteString("\n")
		sb.WriteString(section)
		sb.WriteString("\n")
	}
	sb.WriteString("품질, 가독성, 잠재적 버그를 중심으로 분석하세요.")
	return sb.String(), nil
}

// @AX:NOTE [AUTO]: Review design context is appended only for UI-related files and remains untrusted prompt evidence.
func buildReviewDesignContext(files []string) string {
	cfg, err := config.Load(".")
	if err != nil {
		cfg = config.DefaultFullConfig(".")
	}
	if !design.AnyUIRelatedFile(files, cfg.Design.UIFileGlobs) {
		return "Design context: skipped (non-ui changes)\n"
	}
	if !cfg.Design.InjectOnReview {
		return "Design context: skipped (disabled)\n"
	}
	ctx, err := design.LoadContext(".", design.Options{
		Enabled:         cfg.Design.Enabled,
		Paths:           cfg.Design.Paths,
		MaxContextLines: cfg.Design.MaxContextLines,
		UIFileGlobs:     cfg.Design.UIFileGlobs,
	})
	if err != nil {
		return fmt.Sprintf("Design context: skipped (%v)\n", err)
	}
	if !ctx.Found {
		return fmt.Sprintf("Design context: skipped (not configured)\n%s", ctx.DiagnosticsSummary())
	}
	var sb strings.Builder
	sb.WriteString(ctx.PromptSection())
	sb.WriteString("\nReview UI diffs against this context for palette-role drift, typography hierarchy, component guardrails, layout/responsive regressions, and source-of-truth mismatch.\n")
	return sb.String()
}

// buildSecurePrompt builds the security analysis prompt, including file contents if provided.
func buildSecurePrompt(files []string) (string, error) {
	if len(files) == 0 {
		return "현재 프로젝트의 보안 취약점을 분석해주세요. OWASP Top 10을 기준으로 검토하세요.", nil
	}
	contents, err := buildFileContents(files)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("다음 파일들의 보안 취약점을 분석해주세요:\n\n")
	sb.WriteString(contents)
	sb.WriteString("OWASP Top 10을 기준으로 검토하세요.")
	return sb.String(), nil
}

// flagStringIfChanged returns the flag value only if the flag was explicitly set.
// Returns empty string when using default (not changed).
func flagStringIfChanged(cmd *cobra.Command, name, value string) string {
	if cmd.Flags().Changed(name) {
		return value
	}
	return ""
}

// flagStringSliceIfChanged returns the flag value only if the flag was explicitly set.
// Returns nil when using default (not changed).
func flagStringSliceIfChanged(cmd *cobra.Command, name string, value []string) []string {
	if cmd.Flags().Changed(name) {
		return value
	}
	return nil
}

// resolveRounds returns the effective debate round count.
// Default: 2 for debate strategy when --rounds not specified, 1 for others.
func resolveRounds(strategy string, rounds int) int {
	if rounds > 0 {
		return rounds
	}
	if strategy == "debate" {
		return 2
	}
	return 0
}

// isStdoutTTY returns true if stdout is a terminal device.
// @AX:NOTE: [AUTO] REQ-1 TTY detection — used by auto-detach decision; returns false in CI/pipe contexts
func isStdoutTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// buildProviderConfigs converts provider names to ProviderConfig slice.
// This is the hardcoded fallback used when config is unavailable.
// @AX:NOTE: [AUTO] hardcoded provider registry — add new providers here and in agenticArgs when expanding provider support
func buildProviderConfigs(names []string) []orchestra.ProviderConfig {
	knownProviders := map[string]orchestra.ProviderConfig{
		"claude": {Name: "claude", Binary: "claude", Args: []string{"-p", "--model", "opus", "--effort", "max"}, PaneArgs: []string{"-p", "--model", "opus", "--effort", "max"}, PromptViaArgs: false},
		"codex":  {Name: "codex", Binary: "codex", Args: []string{"exec", "--full-auto", "-m", config.CodexFrontierModel}, PaneArgs: []string{"-m", config.CodexFrontierModel}, PromptViaArgs: false},
		"gemini": {Name: "gemini", Binary: "gemini", Args: []string{"-m", "gemini-3.1-pro-preview", "-p", ""}, PaneArgs: []string{"-m", "gemini-3.1-pro-preview"}, PromptViaArgs: false, StartupTimeout: defaultProviderStartupTimeout("gemini")},
	}

	var result []orchestra.ProviderConfig
	for _, name := range names {
		if p, ok := knownProviders[name]; ok {
			result = append(result, p)
		} else {
			result = append(result, orchestra.ProviderConfig{
				Name:   name,
				Binary: name,
				Args:   []string{},
			})
		}
	}
	return result
}

// defaultProviders returns the hardcoded default provider list.
func defaultProviders() []string {
	return []string{"claude", "codex", "gemini"}
}

func defaultProviderStartupTimeout(name string) time.Duration {
	switch name {
	case "gemini":
		return 20 * time.Second
	default:
		return 0
	}
}

// resolveAndValidateThreshold validates the threshold flag and resolves the final value.
func resolveAndValidateThreshold(orchConf *config.OrchestraConf, configErr error, commandName string, threshold float64) (float64, error) {
	if err := validateThreshold(threshold); err != nil {
		return 0, err
	}
	var resolved float64
	if configErr != nil || orchConf == nil {
		if threshold > 0 {
			resolved = threshold
		} else {
			resolved = 0.66
		}
	} else {
		resolved = resolveThreshold(orchConf, commandName, threshold)
	}
	if err := validateThreshold(resolved); err != nil {
		return 0, fmt.Errorf("resolved threshold invalid: %w", err)
	}
	return resolved, nil
}

// OrchestraFlags holds optional boolean flags for runOrchestraCommand.
// Using a struct instead of variadic any prevents silent breakage when flags are added or reordered.
type OrchestraFlags struct {
	NoDetach       bool
	KeepRelay      bool
	NoJudge        bool
	YieldRounds    bool
	ContextAware   bool
	SubprocessMode bool
	TimeoutChanged bool
}

// isHookModeAvailable checks whether hook-based result collection can be used.
// Returns true only when at least one provider has its hook/plugin registered.
// @AX:NOTE: [AUTO] magic path and string constants — ~/.claude/settings.json, "autopus", "Stop"
func isHookModeAvailable() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	claudeSettings := home + "/.claude/settings.json"
	data, err := os.ReadFile(claudeSettings)
	if err != nil {
		return false
	}
	if strings.Contains(string(data), "autopus") && strings.Contains(string(data), "Stop") {
		return true
	}
	return false
}

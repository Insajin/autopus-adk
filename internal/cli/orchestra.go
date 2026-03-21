package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// buildFileContents reads each file and returns formatted content string.
func buildFileContents(files []string) string {
	var sb strings.Builder
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			sb.WriteString(fmt.Sprintf("--- %s (읽기 실패: %v) ---\n\n", f, err))
			continue
		}
		sb.WriteString(fmt.Sprintf("--- %s ---\n```\n%s\n```\n\n", f, string(content)))
	}
	return sb.String()
}

// newOrchestraCmd creates the orchestra root command.
// @MX:ANCHOR: orchestra 서브커맨드 트리의 루트 — review, plan, secure가 여기서 등록된다.
// @MX:REASON: 세 개의 서브커맨드가 이 함수를 통해 CobraCommand 트리에 추가된다.
func newOrchestraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orchestra",
		Short: "다중 모델 오케스트레이션으로 코드를 분석한다",
		Long: `orchestra는 여러 코딩 CLI를 동시에 실행하여 합의, 파이프라인,
토론, 최속 전략으로 결과를 병합하는 다중 모델 오케스트레이션 엔진입니다.`,
	}

	cmd.AddCommand(newOrchestraReviewCmd())
	cmd.AddCommand(newOrchestraPlanCmd())
	cmd.AddCommand(newOrchestraSecureCmd())

	return cmd
}

// newOrchestraReviewCmd creates the code review subcommand.
func newOrchestraReviewCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
		judge     string
	)

	cmd := &cobra.Command{
		Use:   "review [files...]",
		Short: "여러 모델로 코드를 리뷰한다",
		Long:  "여러 코딩 CLI를 사용하여 지정된 파일을 리뷰하고 결과를 병합합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pass only explicitly set flags; empty string means "use config"
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			prompt := buildReviewPrompt(args)
			return runOrchestraCommand(cmd.Context(), "review", flagStrategy, flagProviders, timeout, judge, prompt)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().StringVar(&judge, "judge", "", "debate 전략에서 최종 판정 프로바이더")

	return cmd
}

// newOrchestraPlanCmd creates the plan subcommand.
func newOrchestraPlanCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
	)

	cmd := &cobra.Command{
		Use:   "plan \"description\"",
		Short: "여러 모델로 구현 계획을 수립한다",
		Long:  "여러 코딩 CLI를 사용하여 기능 구현 계획을 합의 방식으로 수립합니다.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			prompt := fmt.Sprintf("다음 기능 구현 계획을 수립해주세요:\n\n%s", args[0])
			return runOrchestraCommand(cmd.Context(), "plan", flagStrategy, flagProviders, timeout, "", prompt)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")

	return cmd
}

// newOrchestraSecureCmd creates the security analysis subcommand.
func newOrchestraSecureCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
	)

	cmd := &cobra.Command{
		Use:   "secure [files...]",
		Short: "여러 모델로 보안 취약점을 분석한다",
		Long:  "여러 코딩 CLI를 사용하여 지정된 파일의 보안 취약점을 분석합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			prompt := buildSecurePrompt(args)
			return runOrchestraCommand(cmd.Context(), "secure", flagStrategy, flagProviders, timeout, "", prompt)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")

	return cmd
}

// runOrchestraCommand resolves config and runs the orchestration.
// It loads autopus.yaml first, resolves strategy and providers via config,
// and falls back to buildProviderConfigs when config is unavailable.
func runOrchestraCommand(
	ctx context.Context,
	commandName string,
	flagStrategy string,
	flagProviders []string,
	timeout int,
	judge string,
	prompt string,
) error {
	// Attempt to load config; fall back to hardcoded defaults on failure
	orchConf, configErr := loadOrchestraConfig()

	var (
		strategyStr string
		providers   []orchestra.ProviderConfig
	)

	if configErr != nil || orchConf == nil {
		// Config load failed: use CLI flags directly or hardcoded defaults
		strategyStr = flagStrategy
		if strategyStr == "" {
			strategyStr = "consensus"
		}
		names := flagProviders
		if len(names) == 0 {
			names = defaultProviders()
		}
		providers = buildProviderConfigs(names)
	} else {
		// Config loaded: resolve strategy and providers with priority
		strategyStr = resolveStrategy(orchConf, commandName, flagStrategy)
		providers = resolveProviders(orchConf, commandName, flagProviders)
	}

	s := orchestra.Strategy(strategyStr)
	if !s.IsValid() {
		return fmt.Errorf("유효하지 않은 전략: %q (가능한 값: consensus, pipeline, debate, fastest)", strategyStr)
	}

	if len(providers) == 0 {
		return fmt.Errorf("사용 가능한 프로바이더가 없습니다")
	}

	cfg := orchestra.OrchestraConfig{
		Providers:      providers,
		Strategy:       s,
		Prompt:         prompt,
		TimeoutSeconds: timeout,
		JudgeProvider:  judge,
	}

	providerNames := make([]string, len(providers))
	for i, p := range providers {
		providerNames[i] = p.Name
	}
	fmt.Fprintf(os.Stderr, "전략: %s, 프로바이더: %s\n", strategyStr, strings.Join(providerNames, ", "))

	result, err := orchestra.RunOrchestra(ctx, cfg)
	if err != nil {
		return fmt.Errorf("오케스트레이션 실패: %w", err)
	}

	fmt.Printf("%s\n", result.Merged)
	fmt.Fprintf(os.Stderr, "\n요약: %s (총 %s)\n", result.Summary, result.Duration.Round(1e6))
	return nil
}

// buildProviderConfigs converts provider names to ProviderConfig slice.
// This is the hardcoded fallback used when config is unavailable.
func buildProviderConfigs(names []string) []orchestra.ProviderConfig {
	// Known provider mappings: binary + default args
	knownProviders := map[string]orchestra.ProviderConfig{
		// claude: non-interactive mode via stdin
		"claude": {Name: "claude", Binary: "claude", Args: []string{"-p"}, PromptViaArgs: false},
		// codex: quiet mode via stdin
		"codex": {Name: "codex", Binary: "codex", Args: []string{"-q"}, PromptViaArgs: false},
		// gemini: prompt passed as last argument (not stdin)
		"gemini": {Name: "gemini", Binary: "gemini", Args: []string{"-p"}, PromptViaArgs: true},
	}

	var result []orchestra.ProviderConfig
	for _, name := range names {
		if p, ok := knownProviders[name]; ok {
			result = append(result, p)
		} else {
			// Unknown provider: use name as binary
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

// buildReviewPrompt builds the review prompt, including file contents if provided.
func buildReviewPrompt(files []string) string {
	if len(files) == 0 {
		return "현재 프로젝트의 코드를 리뷰해주세요. 품질, 가독성, 잠재적 버그를 중심으로 분석하세요."
	}
	var sb strings.Builder
	sb.WriteString("다음 파일들을 코드 리뷰해주세요:\n\n")
	sb.WriteString(buildFileContents(files))
	sb.WriteString("품질, 가독성, 잠재적 버그를 중심으로 분석하세요.")
	return sb.String()
}

// buildSecurePrompt builds the security analysis prompt, including file contents if provided.
func buildSecurePrompt(files []string) string {
	if len(files) == 0 {
		return "현재 프로젝트의 보안 취약점을 분석해주세요. OWASP Top 10을 기준으로 검토하세요."
	}
	var sb strings.Builder
	sb.WriteString("다음 파일들의 보안 취약점을 분석해주세요:\n\n")
	sb.WriteString(buildFileContents(files))
	sb.WriteString("OWASP Top 10을 기준으로 검토하세요.")
	return sb.String()
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

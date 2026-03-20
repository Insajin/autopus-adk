package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// buildFileContents는 파일 목록의 내용을 읽어 포맷된 문자열로 반환한다.
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

// newOrchestraCmd는 orchestra 커맨드를 생성한다.
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

// newOrchestraReviewCmd는 코드 리뷰 서브커맨드를 생성한다.
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
			prompt := buildReviewPrompt(args)
			return runOrchestraCommand(cmd.Context(), strategy, providers, timeout, judge, prompt)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "debate", "오케스트레이션 전략 (consensus|pipeline|debate|fastest)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", defaultProviders(), "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().StringVar(&judge, "judge", "", "debate 전략에서 최종 판정 프로바이더")

	return cmd
}

// newOrchestraPlanCmd는 계획 생성 서브커맨드를 생성한다.
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
			prompt := fmt.Sprintf("다음 기능 구현 계획을 수립해주세요:\n\n%s", args[0])
			return runOrchestraCommand(cmd.Context(), strategy, providers, timeout, "", prompt)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "consensus", "오케스트레이션 전략 (consensus|pipeline|debate|fastest)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", defaultProviders(), "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")

	return cmd
}

// newOrchestraSecureCmd는 보안 분석 서브커맨드를 생성한다.
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
			prompt := buildSecurePrompt(args)
			return runOrchestraCommand(cmd.Context(), strategy, providers, timeout, "", prompt)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "consensus", "오케스트레이션 전략 (consensus|pipeline|debate|fastest)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", defaultProviders(), "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")

	return cmd
}

// runOrchestraCommand는 오케스트레이션을 실행하고 결과를 출력한다.
func runOrchestraCommand(ctx context.Context, strategyStr string, providerNames []string, timeout int, judge string, prompt string) error {
	s := orchestra.Strategy(strategyStr)
	if !s.IsValid() {
		return fmt.Errorf("유효하지 않은 전략: %q (가능한 값: consensus, pipeline, debate, fastest)", strategyStr)
	}

	providers := buildProviderConfigs(providerNames)
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

	fmt.Fprintf(os.Stderr, "전략: %s, 프로바이더: %s\n", strategyStr, strings.Join(providerNames, ", "))

	result, err := orchestra.RunOrchestra(ctx, cfg)
	if err != nil {
		return fmt.Errorf("오케스트레이션 실패: %w", err)
	}

	fmt.Printf("%s\n", result.Merged)
	fmt.Fprintf(os.Stderr, "\n요약: %s (총 %s)\n", result.Summary, result.Duration.Round(1e6))
	return nil
}

// buildProviderConfigs는 프로바이더 이름 목록을 ProviderConfig 슬라이스로 변환한다.
func buildProviderConfigs(names []string) []orchestra.ProviderConfig {
	// 알려진 프로바이더 이름 → 바이너리 + 기본 인자 매핑
	knownProviders := map[string]orchestra.ProviderConfig{
		// claude: -p 플래그로 비대화형 모드, stdin으로 프롬프트 전달
		"claude": {Name: "claude", Binary: "claude", Args: []string{"-p"}, PromptViaArgs: false},
		// codex: -q 플래그로 quiet 모드, stdin으로 프롬프트 전달
		"codex": {Name: "codex", Binary: "codex", Args: []string{"-q"}, PromptViaArgs: false},
		// gemini: -p 플래그가 프롬프트를 인자로 직접 받음 (stdin 미사용)
		"gemini": {Name: "gemini", Binary: "gemini", Args: []string{"-p"}, PromptViaArgs: true},
	}

	var result []orchestra.ProviderConfig
	for _, name := range names {
		if p, ok := knownProviders[name]; ok {
			result = append(result, p)
		} else {
			// 알 수 없는 프로바이더는 이름을 바이너리로 사용
			result = append(result, orchestra.ProviderConfig{
				Name:   name,
				Binary: name,
				Args:   []string{},
			})
		}
	}
	return result
}

// defaultProviders는 기본 프로바이더 목록을 반환한다.
func defaultProviders() []string {
	return []string{"claude", "codex", "gemini"}
}

// buildReviewPrompt는 리뷰 프롬프트를 생성한다.
// 파일이 지정된 경우 파일 내용을 프롬프트에 포함한다.
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

// buildSecurePrompt는 보안 분석 프롬프트를 생성한다.
// 파일이 지정된 경우 파일 내용을 프롬프트에 포함한다.
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

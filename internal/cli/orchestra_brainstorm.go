package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newOrchestraBrainstormCmd creates the brainstorm subcommand.
func newOrchestraBrainstormCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
		judge     string
	)

	cmd := &cobra.Command{
		Use:   "brainstorm \"feature description\"",
		Short: "여러 모델로 아이디어를 브레인스토밍한다",
		Long: `brainstorm은 여러 코딩 CLI를 동시에 실행하여 기능 아이디어를 다각도로
발산적으로 탐색합니다. SCAMPER 프레임워크와 HMW(How Might We) 질문을 활용하며,
judge 모델이 ICE 점수로 아이디어를 통합하고 증폭합니다.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			prompt := buildBrainstormPrompt(args[0])
			return runOrchestraCommand(cmd.Context(), "brainstorm", flagStrategy, flagProviders, timeout, judge, prompt)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	// @AX:NOTE: [AUTO] magic constant — default timeout 120s matches orchestra consensus SLA
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().StringVar(&judge, "judge", "", "debate 전략에서 최종 판정 프로바이더")

	return cmd
}

// buildBrainstormPrompt builds a structured brainstorming prompt using SCAMPER and HMW.
// The prompt instructs the judge to AUGMENT and INTEGRATE ideas rather than filter them.
func buildBrainstormPrompt(feature string) string {
	return fmt.Sprintf(`You are a divergent-thinking assistant. Generate creative ideas for the following feature using the SCAMPER framework and HMW questions. Do NOT filter — augment and integrate all ideas.

## Feature
%s

## SCAMPER Analysis
For each of the 7 SCAMPER lenses, generate at least 2 concrete ideas:

1. **Substitute**: What components, processes, or data sources could be substituted?
2. **Combine**: What could be combined with this feature to create something new?
3. **Adapt**: What existing patterns or solutions could be adapted here?
4. **Modify/Magnify**: What could be modified, magnified, or minimized?
5. **Put to other uses**: How could this feature be used in unexpected contexts?
6. **Eliminate**: What could be removed to make this simpler or more focused?
7. **Reverse/Rearrange**: What happens if we reverse the flow or rearrange components?

## HMW Questions
Generate 5 "How Might We..." questions that reframe constraints as opportunities.

## Output Format
- List ideas per SCAMPER lens
- List HMW questions
- If you are the judge: INTEGRATE all provider ideas into a merged list, then apply ICE scoring (Impact 1-10, Confidence 1-10, Ease 1-10) to the top 5 merged ideas. Do NOT discard divergent ideas — include them in an appendix.
`, feature)
}

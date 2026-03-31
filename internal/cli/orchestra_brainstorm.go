package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

// newOrchestraBrainstormCmd creates the brainstorm subcommand.
func newOrchestraBrainstormCmd() *cobra.Command {
	var (
		strategy    string
		providers   []string
		timeout     int
		judge       string
		rounds      int
		noDetach    bool
		noJudge      bool
		yieldRounds  bool
		contextAware bool
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
			keepRelay, _ := cmd.Flags().GetBool("keep-relay-output")
			thresholdFlag, _ := cmd.Flags().GetFloat64("threshold")
			prompt := buildBrainstormPrompt(args[0])
			if contextAware {
				prompt = prependProjectContext(prompt)
			}
			resolvedRounds := resolveRounds(flagStrategy, rounds)
			return runOrchestraCommand(cmd.Context(), "brainstorm", flagStrategy, flagProviders, timeout, judge, prompt, resolvedRounds, thresholdFlag, noDetach, keepRelay, noJudge, yieldRounds, contextAware)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest|relay)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	// @AX:NOTE: [AUTO] magic constant — default timeout 120s matches orchestra consensus SLA
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().StringVar(&judge, "judge", "", "debate 전략에서 최종 판정 프로바이더")
	cmd.Flags().Float64("threshold", 0, "consensus 전략 합의 임계값 (0.0-1.0)")
	cmd.Flags().IntVar(&rounds, "rounds", 0, "debate 라운드 수 (1-10, debate 전략 전용)")
	cmd.Flags().BoolVar(&noDetach, "no-detach", false, "Disable auto-detach mode")
	cmd.Flags().Bool("keep-relay-output", false, "relay 전략 실행 후 임시 파일 보존")
	cmd.Flags().BoolVar(&noJudge, "no-judge", false, "Skip judge verdict phase in debate strategy")
	cmd.Flags().BoolVar(&yieldRounds, "yield-rounds", false, "Round 1 후 JSON 출력 및 pane 유지")
	cmd.Flags().BoolVar(&contextAware, "context", false, "Allow providers to read project files (skip topic isolation)")

	return cmd
}

// buildBrainstormPrompt builds a structured brainstorming prompt using SCAMPER and HMW.
// The prompt instructs the judge to AUGMENT and INTEGRATE ideas rather than filter them.
// @AX:NOTE: [AUTO] domain-specific prompt engineering — SCAMPER + HMW + ICE scoring; update when framework changes
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

// contextFiles lists project files to load when --context is set.
var contextFiles = []string{
	"ARCHITECTURE.md",
	".autopus/project/product.md",
	".autopus/project/structure.md",
}

// prependProjectContext adds file path references so providers can read project
// context themselves, instead of injecting full file contents into the prompt.
func prependProjectContext(prompt string) string {
	var existing []string
	for _, f := range contextFiles {
		if _, err := os.Stat(f); err == nil {
			existing = append(existing, f)
		} else {
			log.Printf("[context] skipping %s: %v", f, err)
		}
	}
	if len(existing) == 0 {
		return prompt
	}
	header := "## Project Context\n\nRead the following project files for context before responding:\n"
	for _, f := range existing {
		header += "- " + f + "\n"
	}
	header += "\nUse these files to understand the project architecture, features, and structure.\n\n---\n\n"
	return header + prompt
}

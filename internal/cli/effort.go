// Package cli implements the "auto effort" subcommand group.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// @AX:NOTE [AUTO] subcommand registration point for "auto effort" — extend here to add new effort subcommands
func newEffortCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "effort",
		Short: "Manage effort level detection",
	}
	cmd.AddCommand(newEffortDetectCmd())
	return cmd
}

func newEffortDetectCmd() *cobra.Command {
	var (
		quality    string
		complexity string
		model      string
		agent      string
		format     string
		effortFlag string
	)

	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect recommended effort level based on priority chain",
		Long: `Detect the recommended effort level using the priority chain:
  --effort flag > CLAUDE_CODE_EFFORT_LEVEL env > frontmatter > quality_mode > settings_default

Model/API effort values: low | medium | high | xhigh | max
Opus 5 identifiers: claude-opus-5 | opus (the opus alias resolves to Opus 5 in Claude Code >= 2.1.219).
Claude CLI session-only ultracode uses xhigh plus dynamic workflows (Claude Code >= 2.1.203; agent/team propagation >= 2.1.210).
Fable opt-in identifiers: claude-fable-5 | fable | best (Claude Code >= 2.1.170; entitlement and ZDR restrictions apply).
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			errOut := cmd.ErrOrStderr()
			envValue := ReadEnvEffort()
			for _, warning := range envEffortWarning(envValue) {
				fmt.Fprintln(errOut, warning)
			}

			in := EffortResolveInput{
				EnvValue:       envValue,
				FlagEffort:     effortFlag,
				FlagQuality:    quality,
				FlagComplexity: complexity,
				Model:          model,
			}
			// agent flag is for future frontmatter simulation; record for tracing.
			_ = agent

			result, err := ResolveEffort(in)
			if err != nil {
				fmt.Fprintf(errOut, "%s\n", err.Error())
				os.Exit(2)
			}

			// Emit strip warning to stderr when Haiku stripped effort.
			if result.Effort == EffortStripped && result.Source == EffortSourceQualityMode {
				fmt.Fprintf(errOut, "effort_stripped_model=haiku-4-5\n")
			}

			return writeEffortOutput(cmd, format, result)
		},
	}

	cmd.Flags().StringVar(&quality, "quality", "", "Quality mode preset (ultra|balanced)")
	cmd.Flags().StringVar(&complexity, "complexity", "", "Task complexity hint (low|medium|high)")
	cmd.Flags().StringVar(&model, "model", "", "Model identifier (opus-5|claude-opus-5|opus|opus-4.8|opus-4.7|sonnet-5|sonnet-4.6|haiku-4.5|claude-fable-5|fable|best)")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent name for frontmatter lookup (future use)")
	cmd.Flags().StringVar(&format, "format", "plain", "Output format (plain|json)")
	cmd.Flags().StringVar(&effortFlag, "effort", "", "Explicit model effort or Claude CLI session-only ultracode (overrides quality-mode mapping)")

	return cmd
}

// writeEffortOutput writes the result in the requested format.
func writeEffortOutput(cmd *cobra.Command, format string, result EffortResult) error {
	out := cmd.OutOrStdout()

	switch format {
	case "json":
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("JSON marshal failed: %w", err)
		}
		fmt.Fprintln(out, string(data))
	default:
		// plain: single line "effort=<value>" (empty string for stripped).
		fmt.Fprintf(out, "effort=%s\n", result.Effort)
	}
	return nil
}

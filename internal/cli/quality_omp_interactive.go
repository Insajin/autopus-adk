package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
)

func chooseQualityTarget(cmd *cobra.Command) (string, error) {
	input := cmd.InOrStdin()
	if file, ok := input.(*os.File); ok && file == os.Stdin && !isStdinTTY() {
		return "", fmt.Errorf("interactive quality selection requires a TTY; use an explicit quality or profile command")
	}
	// Reuse one reader across every menu; creating independent buffered
	// readers would discard later answers when input arrives in one batch.
	cmd.SetIn(bufio.NewReader(input))
	fmt.Fprintln(cmd.OutOrStdout(), "What would you like to configure?\n  1) All platforms (quality mode)\n  2) OMP (agent models)")
	fmt.Fprint(cmd.OutOrStdout(), "Choose: ")
	return readQualityChoice(cmd, []string{"global", "omp"})
}

func runOMPQualityInteractive(cmd *cobra.Command, root string, cfg *config.HarnessConfig, deps ompPlatformDependencies) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Choose OMP quality:\n  1) balanced\n  2) ultra")
	fmt.Fprint(out, "Choose: ")
	mode, err := readQualityChoice(cmd, []string{"balanced", "ultra"})
	if err != nil {
		return err
	}
	family := ""
	if _, custom := cfg.RoleModelPolicy.Profiles[mode]; custom {
		fmt.Fprintln(out, "Keeping the models in your custom profile.")
	} else {
		fmt.Fprintln(out, "Choose model family:\n  1) GPT\n  2) Claude")
		fmt.Fprint(out, "Choose: ")
		family, err = readQualityChoice(cmd, []string{"gpt", "claude"})
		if err != nil {
			return err
		}
	}
	opts, err := newOMPProfileApplyOptions(mode, family, nil, false, false)
	if err != nil {
		return err
	}
	runner := deps.newRunner()
	preview, err := planOMPProfile(cmd.Context(), root, opts, runner)
	if err != nil {
		return err
	}
	renderOMPQualitySummary(out, preview)
	if len(preview.Blockers) > 0 {
		return ompProfileUnavailableError{blockers: preview.Blockers}
	}
	fmt.Fprint(out, "Apply these models? [y/N]: ")
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("read apply confirmation: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(out, "Cancelled; no settings changed.")
		return nil
	}
	// The existing transaction revalidates the catalog and configuration
	// after confirmation and retains its normal rollback guarantees.
	if _, err := applyOMPProfile(cmd.Context(), root, opts, runner, deps.activate); err != nil {
		return err
	}
	fmt.Fprintln(out, "OMP models applied. Start a new OMP session to use them.")
	return nil
}

func renderOMPQualitySummary(out io.Writer, preview ompProfileApplyPreviewPayload) {
	fmt.Fprintf(out, "\nOMP %s — model preview (no changes yet)\n", preview.Name)
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "Agent\tModel\tThinking")
	for _, row := range preview.Agents {
		name := row.Agent
		if row.Source == ompProfileSourceAgent {
			name += "*"
		}
		model, thinking := row.EffectiveSelector, row.EffectiveThinking
		if row.Availability != ompProfileAvailabilityAvailable {
			model, thinking = row.RequestedSelector+" (unavailable)", row.RequestedThinking
		}
		fmt.Fprintf(table, "%s\t%s\t%s\n", name, model, thinking)
	}
	_ = table.Flush()
	if len(preview.Persisted.Agents) > 0 {
		fmt.Fprintln(out, "* Your explicit agent overrides are kept.")
	}
	fmt.Fprintln(out, "Global quality and multi-provider review settings stay unchanged.")
}

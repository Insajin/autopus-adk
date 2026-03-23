package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/issue"
)

// newIssueCmd creates the `auto issue` command group.
func newIssueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage auto issue reports",
	}
	cmd.AddCommand(newIssueReportCmd())
	cmd.AddCommand(newIssueListCmd())
	cmd.AddCommand(newIssueSearchCmd())
	return cmd
}

// newIssueReportCmd creates `auto issue report` — collects context, previews,
// and optionally submits a GitHub issue.
func newIssueReportCmd() *cobra.Command {
	var (
		dryRun     bool
		autoSubmit bool
		errMsg     string
		command    string
		exitCode   int
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Create an issue report from an error",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIssueReport(cmd, dryRun, autoSubmit, errMsg, command, exitCode)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the report without submitting")
	cmd.Flags().BoolVar(&autoSubmit, "auto-submit", false, "Submit without confirmation prompt")
	cmd.Flags().StringVar(&errMsg, "error", "", "Error message to report")
	cmd.Flags().StringVar(&command, "command", "", "Command that produced the error")
	cmd.Flags().IntVar(&exitCode, "exit-code", 1, "Exit code of the failed command")

	return cmd
}

// runIssueReport implements the core logic of `auto issue report`.
func runIssueReport(cmd *cobra.Command, dryRun, autoSubmit bool, errMsg, command string, exitCode int) error {
	out := cmd.OutOrStdout()

	ctx := issue.CollectContext(errMsg, command, exitCode)

	report := issue.IssueReport{
		Title:   buildIssueTitle(errMsg, command),
		Context: ctx,
		Labels:  []string{"auto-report"},
	}

	submitter := issue.NewSubmitter(nil)
	report.Hash = submitter.ComputeHash(errMsg, command)

	body, err := issue.FormatMarkdown(report)
	if err != nil {
		return fmt.Errorf("issue report: format: %w", err)
	}

	// Show preview.
	fmt.Fprintln(out, "--- Issue Preview ---")
	fmt.Fprintln(out, body)
	fmt.Fprintln(out, "--- End Preview ---")

	if dryRun {
		fmt.Fprintln(out, "[dry-run] Skipping submission.")
		return nil
	}

	if !autoSubmit {
		if !confirmIssue(cmd, "Submit this issue to GitHub? [y/N] ") {
			fmt.Fprintln(out, "Aborted.")
			return nil
		}
	}

	result, err := submitter.Submit(report, body)
	if err != nil {
		return fmt.Errorf("issue report: submit: %w", err)
	}

	if result.WasDuplicate {
		fmt.Fprintf(out, "Duplicate found. Added comment to: %s\n", result.IssueURL)
	} else {
		fmt.Fprintf(out, "Issue created: %s\n", result.IssueURL)
	}
	return nil
}

// buildIssueTitle constructs a short issue title from error and command.
func buildIssueTitle(errMsg, command string) string {
	if command != "" && errMsg != "" {
		short := errMsg
		if len(short) > 60 {
			short = short[:60] + "..."
		}
		return fmt.Sprintf("[auto] %s: %s", command, short)
	}
	if errMsg != "" {
		short := errMsg
		if len(short) > 72 {
			short = short[:72] + "..."
		}
		return fmt.Sprintf("[auto] %s", short)
	}
	return "[auto] issue report"
}

// confirmIssue reads a yes/no answer from the user via stdin.
func confirmIssue(cmd *cobra.Command, prompt string) bool {
	_, _ = fmt.Fprint(cmd.OutOrStdout(), prompt)
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if scanner.Scan() {
		ans := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return ans == "y" || ans == "yes"
	}
	return false
}

// newIssueListCmd creates `auto issue list` — lists auto-report labeled issues.
func newIssueListCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List auto-report labeled issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIssueList(cmd, repo)
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository (owner/repo)")
	return cmd
}

// runIssueList calls gh issue list filtering by the auto-report label.
func runIssueList(cmd *cobra.Command, repo string) error {
	ghArgs := []string{"issue", "list", "--label", "auto-report"}
	if repo != "" {
		ghArgs = append([]string{"--repo", repo}, ghArgs...)
	}
	return runGHCmd(cmd, ghArgs...)
}

// newIssueSearchCmd creates `auto issue search` — searches issues by query.
func newIssueSearchCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search issues by query",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIssueSearch(cmd, repo, strings.Join(args, " "))
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository (owner/repo)")
	return cmd
}

// runIssueSearch calls gh issue list with a search query.
func runIssueSearch(cmd *cobra.Command, repo, query string) error {
	ghArgs := []string{"issue", "list", "--search", query}
	if repo != "" {
		ghArgs = append([]string{"--repo", repo}, ghArgs...)
	}
	return runGHCmd(cmd, ghArgs...)
}

// runGHCmd executes a gh subcommand and streams output to cmd's stdout/stderr.
func runGHCmd(cmd *cobra.Command, args ...string) error {
	c := exec.Command("gh", args...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		return fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

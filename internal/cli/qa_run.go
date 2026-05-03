package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func newQARunCmd() *cobra.Command {
	var opts qaRunOptions
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute project QA journeys and write QAMESH evidence",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQARun(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Profile, "profile", "standalone", "QA profile")
	cmd.Flags().StringVar(&opts.Lane, "lane", "fast", "QA lane")
	cmd.Flags().StringVar(&opts.JourneyID, "journey", "", "Journey id filter")
	cmd.Flags().StringVar(&opts.AdapterID, "adapter", "", "Adapter id filter")
	cmd.Flags().StringVar(&opts.Output, "output", "", "Run output root")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Plan without executing adapters")
	cmd.Flags().StringVar(&opts.FeedbackTo, "feedback-to", "", "Feedback target")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

type qaRunOptions struct {
	ProjectDir string
	Profile    string
	Lane       string
	JourneyID  string
	AdapterID  string
	Output     string
	DryRun     bool
	FeedbackTo string
	JSONOut    bool
	Format     string
}

func runQARun(cmd *cobra.Command, opts qaRunOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	if opts.Output != "" {
		if err := rejectGeneratedQAOutput("output", opts.Output); err != nil {
			return err
		}
	}
	result, err := qarun.Execute(qarun.Options{
		ProjectDir: opts.ProjectDir,
		Profile:    opts.Profile,
		Lane:       opts.Lane,
		JourneyID:  opts.JourneyID,
		AdapterID:  opts.AdapterID,
		Output:     opts.Output,
		DryRun:     opts.DryRun,
		FeedbackTo: opts.FeedbackTo,
	})
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(cmd, jsonStatusError, err, "qa_run_failed", result, nil, nil)
		}
		return err
	}
	status := jsonStatusOK
	if result.Status == "warning" {
		status = jsonStatusWarn
	}
	if jsonMode {
		return writeJSONResult(cmd, status, result, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", result.Status, result.RunIndexPath)
	return nil
}

package cli

import (
	"github.com/spf13/cobra"

	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func newQAExploreCmd() *cobra.Command {
	var opts qaRunOptions
	opts.Lane = "gui-explore"
	cmd := &cobra.Command{
		Use:   "explore",
		Short: "Execute explicit GUI exploration journeys and write QAMESH evidence",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAExplore(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Profile, "profile", "local", "QA profile")
	cmd.Flags().StringVar(&opts.Lane, "lane", "gui-explore", "QA lane")
	cmd.Flags().StringVar(&opts.JourneyID, "journey", "", "Journey id filter")
	cmd.Flags().StringVar(&opts.Output, "output", "", "Run output root")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Plan without executing adapters")
	cmd.Flags().StringVar(&opts.FeedbackTo, "feedback-to", "", "Feedback target")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runQAExplore(cmd *cobra.Command, opts qaRunOptions) error {
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
		AdapterID:  "gui-explore",
		Output:     opts.Output,
		DryRun:     opts.DryRun,
		FeedbackTo: opts.FeedbackTo,
	})
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(cmd, jsonStatusError, err, "qa_explore_failed", result, nil, nil)
		}
		writeQARunSummary(cmd, result.Status, result.RunIndexPath, "", opts.ProjectDir)
		return err
	}
	status := jsonStatusOK
	if result.Status == "warning" {
		status = jsonStatusWarn
	}
	if jsonMode {
		return writeJSONResult(cmd, status, result, nil, nil)
	}
	writeQARunSummary(cmd, result.Status, result.RunIndexPath, "", opts.ProjectDir)
	return nil
}

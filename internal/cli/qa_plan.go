package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func newQAPlanCmd() *cobra.Command {
	var opts qaPlanOptions
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan project QA journeys without executing commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAPlan(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Profile, "profile", "standalone", "QA profile")
	cmd.Flags().StringVar(&opts.Lane, "lane", "fast", "QA lane")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

type qaPlanOptions struct {
	ProjectDir string
	Profile    string
	Lane       string
	JSONOut    bool
	Format     string
}

func runQAPlan(cmd *cobra.Command, opts qaPlanOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	plan, err := qarun.BuildPlan(qarun.Options{
		ProjectDir: opts.ProjectDir,
		Profile:    opts.Profile,
		Lane:       opts.Lane,
	})
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_plan_failed", map[string]any{"project_dir": opts.ProjectDir})
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, plan, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "lane=%s journeys=%d adapters=%d\n", plan.SelectedLane, len(plan.SelectedJourneys), len(plan.SelectedAdapters))
	return nil
}

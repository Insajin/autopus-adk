package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	qascaffold "github.com/insajin/autopus-adk/pkg/qa/scaffold"
)

func newQAInitCmd() *cobra.Command {
	var opts qaInitOptions
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create project-local QA Journey Pack starters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAInit(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

type qaInitOptions struct {
	ProjectDir string
	JSONOut    bool
	Format     string
}

func runQAInit(cmd *cobra.Command, opts qaInitOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	result, err := qascaffold.Init(qascaffold.Options{
		ProjectDir: opts.ProjectDir,
	})
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_init_failed", map[string]any{"project_dir": opts.ProjectDir})
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, result, nil, nil)
	}
	for _, created := range result.Created {
		fmt.Fprintf(cmd.OutOrStdout(), "created %s (%s)\n", created.Path, created.Reason)
	}
	for _, skipped := range result.Skipped {
		fmt.Fprintf(cmd.OutOrStdout(), "skipped %s (%s)\n", skipped.Path, skipped.Reason)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", warning)
	}
	for _, step := range result.NextSteps {
		fmt.Fprintf(cmd.OutOrStdout(), "next: %s\n", step)
	}
	return nil
}

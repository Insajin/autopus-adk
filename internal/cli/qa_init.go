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
		Short: "Initialize release-ready QAMESH QA scaffolding",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAInit(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().BoolVar(&opts.LocalOnly, "local-only", false, "Create Journey Pack starters only; skip release lanes and workflow scaffold")
	cmd.Flags().BoolVar(&opts.Release, "release", false, "Create release-gate starter lanes such as canary-explicit")
	cmd.Flags().StringVar(&opts.Workflow, "workflow", "", "Optional release workflow scaffold (none|github-actions)")
	_ = cmd.Flags().MarkHidden("release")
	_ = cmd.Flags().MarkHidden("workflow")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func newQABootstrapCmd() *cobra.Command {
	var opts qaInitOptions
	cmd := &cobra.Command{
		Use:    "bootstrap",
		Short:  "Bootstrap Journey Packs and a release QA workflow",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Release = true
			opts.Workflow = "github-actions"
			return runQAScaffold(cmd, opts, "qa_bootstrap_failed")
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

type qaInitOptions struct {
	ProjectDir string
	LocalOnly  bool
	Release    bool
	Workflow   string
	JSONOut    bool
	Format     string
}

func runQAInit(cmd *cobra.Command, opts qaInitOptions) error {
	if opts.LocalOnly {
		opts.Release = false
		opts.Workflow = "none"
	} else {
		opts.Release = true
		if opts.Workflow == "" {
			opts.Workflow = "github-actions"
		}
	}
	return runQAScaffold(cmd, opts, "qa_init_failed")
}

func runQAScaffold(cmd *cobra.Command, opts qaInitOptions, errorCode string) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	result, err := qascaffold.Init(qascaffold.Options{
		ProjectDir:         opts.ProjectDir,
		ProjectDirExplicit: cmd.Flags().Changed("project-dir"),
		Release:            opts.Release,
		Workflow:           opts.Workflow,
	})
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, errorCode, map[string]any{"project_dir": opts.ProjectDir})
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

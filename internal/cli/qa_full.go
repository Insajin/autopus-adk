package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
	qascaffold "github.com/insajin/autopus-adk/pkg/qa/scaffold"
)

type qaFullOptions struct {
	ProjectDir       string
	Profile          string
	Output           string
	RunOutputRoot    string
	Run              bool
	Bootstrap        bool
	RuntimeProviders []string
	JSONOut          bool
	Format           string
}

func newQAFullCmd() *cobra.Command {
	var opts qaFullOptions
	cmd := &cobra.Command{
		Use:   "full",
		Short: "Plan or run the full project QA gate with one command",
		Long:  "Plan the full QAMESH release-style QA gate by default. Add --run to execute the selected project-local lanes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAFull(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Profile, "profile", "prelaunch", "Full QA profile")
	cmd.Flags().StringVar(&opts.Output, "output", "", "Release output root")
	cmd.Flags().StringVar(&opts.RunOutputRoot, "run-output", "", "QAMESH run output root")
	cmd.Flags().BoolVar(&opts.Run, "run", false, "Execute the full gate instead of planning only")
	cmd.Flags().BoolVar(&opts.Bootstrap, "bootstrap", false, "Create safe QA starter files before planning or running")
	cmd.Flags().StringArrayVar(&opts.RuntimeProviders, "runtime-provider", nil, "Desktop observation runtime provider (local or orca; exactly one)")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runQAFull(cmd *cobra.Command, opts qaFullOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	runtimeProvider, err := parseQARuntimeProvider(cmd, jsonMode, opts.RuntimeProviders)
	if err != nil {
		return err
	}
	if err := requireQARuntimeProvider(cmd, jsonMode, runtimeProvider, projectRequiresQARuntimeProvider(opts.ProjectDir)); err != nil {
		return err
	}
	if opts.Output != "" {
		if err := rejectGeneratedQAOutput("output", opts.Output); err != nil {
			return err
		}
	}
	if opts.RunOutputRoot != "" {
		if err := rejectGeneratedQAOutput("run-output", opts.RunOutputRoot); err != nil {
			return err
		}
	}
	selection, err := resolveQAFullProjectSelection(cmd, opts)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_full_project_failed", map[string]any{"project_dir": opts.ProjectDir})
	}
	if selection != nil && selection.Payload != nil {
		if jsonMode {
			return writeJSONResult(cmd, jsonStatusWarn, selection.Payload, nil, nil)
		}
		writeQAFullText(cmd, *selection.Payload)
		return nil
	}
	if selection != nil && selection.ProjectDir != "" {
		opts.ProjectDir = selection.ProjectDir
	}
	if err := requireQARuntimeProvider(cmd, jsonMode, runtimeProvider, projectRequiresQARuntimeProvider(opts.ProjectDir)); err != nil {
		return err
	}
	var bootstrap *qascaffold.Result
	if opts.Bootstrap {
		result, err := qascaffold.Init(qascaffold.Options{
			ProjectDir:         opts.ProjectDir,
			ProjectDirExplicit: true,
			Release:            true,
			Workflow:           "none",
		})
		if err != nil {
			return qaCommandError(cmd, jsonMode, err, "qa_full_bootstrap_failed", map[string]any{"project_dir": opts.ProjectDir})
		}
		bootstrap = &result
	}
	releaseOpts := qarelease.Options{
		ProjectDir:      opts.ProjectDir,
		Profile:         opts.Profile,
		Output:          opts.Output,
		RunOutputRoot:   opts.RunOutputRoot,
		Command:         qaFullCommandString(opts, jsonMode),
		RuntimeProvider: runtimeProvider,
	}
	domain := loadQAFullDomainReadiness(opts.ProjectDir)
	if opts.Run {
		return runQAFullExecution(cmd, releaseOpts, opts, domain, bootstrap, jsonMode)
	}
	plan, err := qarelease.BuildPlan(releaseOpts)
	if err != nil {
		payload := qaFullPayload{SchemaVersion: qaFullSchemaVersion, Mode: "plan", Profile: opts.Profile, ProjectDir: opts.ProjectDir, Bootstrap: bootstrap, DomainReadiness: domain}
		return writeJSONResultAndExit(cmd, jsonStatusError, err, "qa_full_plan_failed", payload, nil, nil)
	}
	payload := buildQAFullPlanPayload(opts, plan, domain, bootstrap)
	status := jsonStatusOK
	if payload.Summary.Status != "ready" {
		status = jsonStatusWarn
	}
	if jsonMode {
		return writeJSONResult(cmd, status, payload, nil, nil)
	}
	writeQAFullText(cmd, payload)
	return nil
}

func runQAFullExecution(cmd *cobra.Command, releaseOpts qarelease.Options, opts qaFullOptions, domain qaFullDomainReadiness, bootstrap *qascaffold.Result, jsonMode bool) error {
	result, err := qarelease.Execute(releaseOpts)
	payload := buildQAFullRunPayload(opts, result, domain, bootstrap)
	if err != nil {
		code := "qa_full_failed"
		if errors.Is(err, qarelease.ErrReleaseBlocked) {
			code = "qa_full_blocked"
		}
		if jsonMode {
			return writeJSONResultAndExit(cmd, jsonStatusError, err, code, payload, nil, nil)
		}
		return err
	}
	status := jsonStatusOK
	if result.Status == qarelease.GateStatusWarn || payload.Summary.Status == "setup_gap" {
		status = jsonStatusWarn
	}
	if jsonMode {
		return writeJSONResult(cmd, status, payload, nil, nil)
	}
	writeQAFullText(cmd, payload)
	return nil
}

type qaFullProjectSelection struct {
	ProjectDir string
	Payload    *qaFullPayload
}

func resolveQAFullProjectSelection(cmd *cobra.Command, opts qaFullOptions) (*qaFullProjectSelection, error) {
	if cmd.Flags().Changed("project-dir") {
		return nil, nil
	}
	if qascaffold.HasQAScaffoldSignals(opts.ProjectDir) {
		return nil, nil
	}
	targets, hasChildRepos, err := qascaffold.DetectWorkspaceQATargets(opts.ProjectDir)
	if err != nil || !hasChildRepos {
		return nil, err
	}
	if len(targets) == 1 {
		return &qaFullProjectSelection{ProjectDir: targets[0].ProjectDir}, nil
	}
	payload := buildQAFullSelectProjectPayload(opts, targets, hasChildRepos)
	return &qaFullProjectSelection{Payload: &payload}, nil
}

func writeQAFullText(cmd *cobra.Command, payload qaFullPayload) {
	fmt.Fprintf(cmd.OutOrStdout(), "full qa %s profile=%s project=%s\n", payload.Summary.Status, payload.Profile, payload.ProjectDir)
	fmt.Fprintf(cmd.OutOrStdout(), "mode=%s action=%s lanes=%d journeys=%d setup_gaps=%d domain_scenarios=%d\n", payload.Mode, payload.Summary.Action, len(payload.Summary.SelectedLanes), payload.Summary.JourneyPackCount, payload.Summary.SetupGapCount, payload.Summary.DomainScenarioCount)
	if payload.QAPolicy.Orchestrator != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "qa_policy=%s adapters=%s choices=%s\n", payload.QAPolicy.Orchestrator, strings.Join(payload.QAPolicy.RunnerAdapters, ","), strings.Join(payload.QAPolicy.UserChoiceRequiredFor, ","))
	}
	if payload.Bootstrap != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "bootstrap=%s created=%d skipped=%d\n", payload.Bootstrap.Status, len(payload.Bootstrap.Created), len(payload.Bootstrap.Skipped))
	}
	for _, candidate := range payload.ProjectCandidates {
		fmt.Fprintf(cmd.OutOrStdout(), "candidate: %s score=%d reasons=%s\n", candidate.ProjectDir, candidate.Score, candidate.Reason)
	}
	for _, next := range payload.NextCommands {
		fmt.Fprintf(cmd.OutOrStdout(), "next: %s\n", next)
	}
}

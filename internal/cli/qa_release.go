package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
)

func newQAReleaseCmd() *cobra.Command {
	var opts qaReleaseOptions
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Plan or execute QAMESH release gate lanes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQARelease(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Profile, "profile", "prelaunch", "Release profile")
	cmd.Flags().StringVar(&opts.Output, "output", "", "Release output root")
	cmd.Flags().StringVar(&opts.RunOutputRoot, "run-output", "", "QAMESH run output root")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Plan without executing lanes")
	cmd.Flags().BoolVar(&opts.Roadmap, "roadmap", false, "Print release QA roadmap")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

type qaReleaseOptions struct {
	ProjectDir    string
	Profile       string
	Output        string
	RunOutputRoot string
	DryRun        bool
	Roadmap       bool
	JSONOut       bool
	Format        string
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: CLI release command is the user-facing release gate orchestration boundary.
func runQARelease(cmd *cobra.Command, opts qaReleaseOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	if opts.Output != "" {
		if err := rejectGeneratedQAOutput("output", opts.Output); err != nil {
			return err
		}
	}
	command := releaseCommandString(opts, jsonMode)
	if opts.Roadmap {
		payload := qarelease.RoadmapAt(time.Now().UTC().Format(time.RFC3339Nano))
		if jsonMode {
			return writeJSONResult(cmd, jsonStatusOK, payload, nil, nil)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s lanes=%d\n", payload.SchemaVersion, len(payload.Lanes))
		return nil
	}
	if err := qarelease.ValidateProfile(opts.Profile); err != nil {
		return qaReleaseJSONError(cmd, jsonMode, err, "qa_release_invalid_profile", map[string]any{"profile": opts.Profile})
	}
	releaseOpts := qarelease.Options{
		ProjectDir:    opts.ProjectDir,
		Profile:       opts.Profile,
		Output:        opts.Output,
		RunOutputRoot: opts.RunOutputRoot,
		Command:       command,
		DryRun:        opts.DryRun,
	}
	if opts.DryRun {
		plan, err := qarelease.BuildPlan(releaseOpts)
		if err != nil {
			return qaReleaseJSONError(cmd, jsonMode, err, "qa_release_plan_failed", plan)
		}
		if jsonMode {
			return writeJSONResult(cmd, jsonStatusOK, plan, nil, nil)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s profile=%s lanes=%d\n", plan.SchemaVersion, plan.Profile, len(plan.SelectedLanes))
		return nil
	}
	payload, err := qarelease.Execute(releaseOpts)
	if err != nil {
		code := "qa_release_failed"
		if errors.Is(err, qarelease.ErrReleaseBlocked) {
			code = "qa_release_blocked"
		}
		return qaReleaseJSONError(cmd, jsonMode, err, code, payload)
	}
	status := jsonStatusOK
	if payload.Status == qarelease.GateStatusWarn {
		status = jsonStatusWarn
	}
	if jsonMode {
		return writeJSONResult(cmd, status, payload, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", payload.Status, payload.ReleaseIndexPath)
	return nil
}

func qaReleaseJSONError(cmd *cobra.Command, jsonMode bool, err error, code string, data any) error {
	if jsonMode {
		return writeJSONResultAndExit(cmd, jsonStatusError, err, code, data, nil, nil)
	}
	return err
}

func releaseCommandString(opts qaReleaseOptions, jsonMode bool) string {
	parts := []string{"auto", "qa", "release"}
	if opts.Profile != "" {
		parts = append(parts, "--profile", opts.Profile)
	}
	if opts.DryRun {
		parts = append(parts, "--dry-run")
	}
	if opts.Roadmap {
		parts = append(parts, "--roadmap")
	}
	if jsonMode {
		parts = append(parts, "--format", "json")
	}
	return strings.Join(parts, " ")
}

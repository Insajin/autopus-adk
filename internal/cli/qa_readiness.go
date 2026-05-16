package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/qa/readiness"
)

type qaReadinessOptions struct {
	WorkspaceRoot string
	RepoRoot      string
	WorkspaceID   string
	RepoID        string
	RunIndexPath  string
	ReleasePath   string
	JSONOut       bool
	Format        string
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: public `auto qa readiness` command emits the projection consumed by Autopus surfaces.
// @AX:REASON: CLI users, backend ingestion, frontend CEO cards, and desktop cards rely on the flags, JSON mode, and error codes staying stable.
func newQAReadinessCmd() *cobra.Command {
	var opts qaReadinessOptions
	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "Project redacted QAMESH readiness from run and release indexes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAReadiness(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceRoot, "workspace-root", ".", "Workspace root containing QAMESH evidence")
	cmd.Flags().StringVar(&opts.RepoRoot, "repo-root", ".", "Repository root for ownership validation")
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace-id", "", "Expected workspace id")
	cmd.Flags().StringVar(&opts.RepoID, "repo-id", "", "Expected repository id")
	cmd.Flags().StringVar(&opts.RunIndexPath, "run-index", "", "QAMESH run index path")
	cmd.Flags().StringVar(&opts.ReleasePath, "release-index", "", "QAMESH release index path")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runQAReadiness(cmd *cobra.Command, opts qaReadinessOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	for name, value := range map[string]string{
		"workspace-id":  opts.WorkspaceID,
		"repo-id":       opts.RepoID,
		"run-index":     opts.RunIndexPath,
		"release-index": opts.ReleasePath,
	} {
		if err := requireFlag(name, value); err != nil {
			return qaCommandError(cmd, jsonMode, err, "qa_readiness_invalid_flags", nil)
		}
	}
	result, err := readiness.Project(context.Background(), readiness.Input{
		WorkspaceRoot: opts.WorkspaceRoot,
		RepoRoot:      opts.RepoRoot,
		WorkspaceID:   opts.WorkspaceID,
		RepoID:        opts.RepoID,
		RunIndexPath:  opts.RunIndexPath,
		ReleasePath:   opts.ReleasePath,
	})
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_readiness_blocked", result)
	}
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: reference consumers enumerate downstream projection owners for contract drift checks.
	result.Projection.ReferenceConsumers = []string{"Autopus", "autopus-desktop"}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, result.Projection, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", result.Projection.ReleaseVerdict, result.Projection.LastRunTime)
	return nil
}

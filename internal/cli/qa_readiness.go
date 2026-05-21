package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/qa/readiness"
)

type qaReadinessOptions struct {
	WorkspaceRoot      string
	RepoRoot           string
	WorkspaceID        string
	RepoID             string
	RunIndexPath       string
	ReleasePath        string
	ReferenceConsumers []string
	JSONOut            bool
	Format             string
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: public `auto qa readiness` command emits portable read-only QAMESH projections.
// @AX:REASON: CLI users and downstream project surfaces rely on the flags, JSON mode, and error codes staying stable.
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
	cmd.Flags().StringArrayVar(&opts.ReferenceConsumers, "reference-consumer", nil, "Optional downstream consumer name; may be repeated")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runQAReadiness(cmd *cobra.Command, opts qaReadinessOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	opts, err = resolveReadinessDefaults(opts)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_readiness_invalid_flags", nil)
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
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: reference consumers are caller-owned metadata, not ADK product defaults.
	result.Projection.ReferenceConsumers = append([]string(nil), opts.ReferenceConsumers...)
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, result.Projection, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", result.Projection.ReleaseVerdict, result.Projection.LastRunTime)
	return nil
}

func resolveReadinessDefaults(opts qaReadinessOptions) (qaReadinessOptions, error) {
	if opts.WorkspaceRoot == "" {
		opts.WorkspaceRoot = "."
	}
	var err error
	if opts.RunIndexPath == "" {
		opts.RunIndexPath, err = latestIndexPath(opts.WorkspaceRoot, filepath.Join(".autopus", "qa", "runs"), "run-index.json")
		if err != nil {
			return opts, err
		}
	}
	if opts.ReleasePath == "" {
		opts.ReleasePath, err = latestIndexPath(opts.WorkspaceRoot, filepath.Join(".autopus", "qa", "releases"), "release-index.json")
		if err != nil {
			return opts, err
		}
	}
	if opts.WorkspaceID == "" || opts.RepoID == "" || opts.RepoRoot == "." || opts.RepoRoot == "" {
		ref, err := readReadinessWorkspaceRef(opts.ReleasePath)
		if err != nil {
			return opts, err
		}
		if opts.WorkspaceID == "" {
			opts.WorkspaceID = ref.WorkspaceID
		}
		if opts.RepoID == "" {
			opts.RepoID = ref.RepoID
		}
		if opts.RepoRoot == "." || opts.RepoRoot == "" {
			opts.RepoRoot = filepath.Join(opts.WorkspaceRoot, filepath.FromSlash(ref.RepoRoot))
		}
	}
	return opts, nil
}

func latestIndexPath(workspaceRoot, relDir, filename string) (string, error) {
	pattern := filepath.Join(workspaceRoot, relDir, "*", filename)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	sort.Slice(matches, func(i, j int) bool {
		return modTime(matches[i]).After(modTime(matches[j]))
	})
	return matches[0], nil
}

func modTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

type readinessWorkspaceRef struct {
	WorkspaceID string `json:"workspace_id"`
	RepoID      string `json:"repo_id"`
	RepoRoot    string `json:"repo_root"`
}

func readReadinessWorkspaceRef(path string) (readinessWorkspaceRef, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return readinessWorkspaceRef{}, err
	}
	var doc struct {
		Workspace readinessWorkspaceRef `json:"workspace"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return readinessWorkspaceRef{}, err
	}
	return doc.Workspace, nil
}

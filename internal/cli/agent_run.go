package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// validTaskID matches safe task IDs: alphanumeric, hyphens, underscores only.
var validTaskID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// taskContext represents the input context for an agent task.
type taskContext struct {
	TaskID      string `yaml:"task_id"`
	Description string `yaml:"description"`
}

// taskResult represents the output result of an agent task.
type taskResult struct {
	TaskID    string `yaml:"task_id"`
	Status    string `yaml:"status"`
	Timestamp string `yaml:"timestamp"`
}

// newAgentRunSubCmd creates the `auto agent run <task-id>` subcommand.
func newAgentRunSubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <task-id>",
		Short: "Run an agent task",
		Long:  "Execute a single pipeline task, reading context from .autopus/runs/<task-id>/context.yaml and writing results to result.yaml.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentTask(args[0])
		},
	}
	return cmd
}

// runAgentTask reads context.yaml for the given task ID and writes result.yaml upon completion.
func runAgentTask(taskID string) error {
	// Validate task ID to prevent path traversal (V-001).
	if !validTaskID.MatchString(taskID) {
		return fmt.Errorf("invalid task ID %q: must be alphanumeric with hyphens or underscores", taskID)
	}
	runsDir := filepath.Join(".autopus", "runs", taskID)
	contextPath := filepath.Join(runsDir, "context.yaml")

	// Read and parse context.yaml.
	data, err := os.ReadFile(contextPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task context not found: %s", taskID)
		}
		return fmt.Errorf("read context for %s: %w", taskID, err)
	}

	var ctx taskContext
	if err := yaml.Unmarshal(data, &ctx); err != nil {
		return fmt.Errorf("parse context for %s: %w", taskID, err)
	}

	// Execute the task (placeholder — real execution will be added in T8).
	_ = ctx

	// Write result.yaml.
	result := taskResult{
		TaskID:    taskID,
		Status:    "success",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	resultData, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result for %s: %w", taskID, err)
	}

	resultPath := filepath.Join(runsDir, "result.yaml")
	if err := os.WriteFile(resultPath, resultData, 0o600); err != nil {
		return fmt.Errorf("write result for %s: %w", taskID, err)
	}

	return nil
}

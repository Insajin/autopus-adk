package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/qa/adapter"
)

func newQAAdaptersCmd() *cobra.Command {
	var (
		projectDir string
		jsonOut    bool
		format     string
	)
	cmd := &cobra.Command{
		Use:   "adapters",
		Short: "List QAMESH adapter registry metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAAdapters(cmd, projectDir, jsonOut, format)
		},
	}
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "Project directory")
	addJSONFlags(cmd, &jsonOut, &format)
	return cmd
}

func runQAAdapters(cmd *cobra.Command, projectDir string, jsonOut bool, format string) error {
	jsonMode, err := resolveJSONMode(jsonOut, format)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"project_dir": projectDir,
		"adapters":    adapter.WithSetupGaps(),
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, payload, nil, nil)
	}
	for _, item := range adapter.WithSetupGaps() {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", item.ID)
	}
	return nil
}

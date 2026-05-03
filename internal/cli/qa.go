package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newQACmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "qa",
		Short:         "Normalize QA evidence and generate repair prompts",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newQAPlanCmd())
	cmd.AddCommand(newQAAdaptersCmd())
	cmd.AddCommand(newQARunCmd())
	cmd.AddCommand(newQAEvidenceCmd())
	cmd.AddCommand(newQAFeedbackCmd())
	return cmd
}

func requireFlag(name, value string) error {
	if value == "" {
		return fmt.Errorf("missing --%s", name)
	}
	return nil
}

func rejectGeneratedQAOutput(name, value string) error {
	rel := strings.ToLower(filepath.ToSlash(filepath.Clean(value)))
	for _, denied := range []string{".codex", ".claude", ".gemini", ".opencode", ".autopus/plugins"} {
		if rel == denied || strings.HasPrefix(rel, denied+"/") || strings.Contains(rel, "/"+denied+"/") || strings.HasSuffix(rel, "/"+denied) {
			return fmt.Errorf("--%s may not target generated surface %s", name, denied)
		}
	}
	return nil
}

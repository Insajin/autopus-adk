package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/design"
)

func newDesignFigmaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "figma",
		Short: "Inspect optional Figma and Code Connect design refs",
	}
	cmd.AddCommand(newDesignFigmaAuditCmd())
	cmd.AddCommand(newDesignFigmaFetchCmd())
	return cmd
}

func newDesignFigmaAuditCmd() *cobra.Command {
	var dir string
	var format string
	var output string
	var maxRefs int
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit Figma refs and Code Connect readiness without network access",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveDir(dir)
			if err != nil {
				return err
			}
			audit, err := design.AuditFigma(root, maxRefs)
			if err != nil {
				return err
			}
			data, err := renderDesignOutput(format, audit)
			if err != nil {
				return err
			}
			return writeOrPrint(cmd, output, data)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "project root directory")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown or json")
	cmd.Flags().StringVar(&output, "output", "", "write output to a file instead of stdout")
	cmd.Flags().IntVar(&maxRefs, "max-refs", 30, "maximum refs to include")
	return cmd
}

func newDesignFigmaFetchCmd() *cobra.Command {
	var dir string
	var format string
	var output string
	var maxRefs int
	var apiBase string
	var depth int
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch compact Figma node metadata when FIGMA_ACCESS_TOKEN is available",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveDir(dir)
			if err != nil {
				return err
			}
			report, err := design.FetchFigmaNodes(context.Background(), root, design.FigmaFetchOptions{
				APIBaseURL: apiBase,
				MaxRefs:    maxRefs,
				Depth:      depth,
			})
			if err != nil {
				return err
			}
			data, err := renderDesignOutput(format, report)
			if err != nil {
				return err
			}
			return writeOrPrint(cmd, output, data)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "project root directory")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown or json")
	cmd.Flags().StringVar(&output, "output", "", "write output to a file instead of stdout")
	cmd.Flags().IntVar(&maxRefs, "max-refs", 30, "maximum Figma refs to fetch")
	cmd.Flags().StringVar(&apiBase, "api-base", design.DefaultFigmaAPIBaseURL, "Figma API base URL")
	cmd.Flags().IntVar(&depth, "depth", 1, "Figma node tree depth to request")
	return cmd
}

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/insajin/autopus-adk/pkg/workflow"
	"github.com/insajin/autopus-adk/templates"
)

const (
	workflowSchemaEmbedPath   = "workflows/route_a.schema.json"
	workflowContractEmbedPath = "workflows/route_a.md"
	workflowJSEmbedPath       = "claude/workflows/route_a.workflow.js.tmpl"

	workflowContractDisplayPath = "content/workflows/route_a.md"
	workflowSchemaDisplayPath   = "content/workflows/route_a.schema.json"
)

// newWorkflowRenderCmd loads the canonical manifest from the embedded content/
// and templates/ filesystems and emits the dry-run report (phase order, gate
// verdict source, manifest/schema paths, deterministic prompt-manifest hash, and
// the generated JS) without executing any agent (REQ-010, S7/S11).
func newWorkflowRenderCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:           "render",
		Short:         "Render the generated workflow JS, manifest, schema, and prompt-manifest hash",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			schemaBytes, err := content.FS.ReadFile(workflowSchemaEmbedPath)
			if err != nil {
				return fmt.Errorf("read embedded workflow schema: %w", err)
			}
			schema, err := workflow.ParseSchema(schemaBytes)
			if err != nil {
				return fmt.Errorf("parse workflow schema: %w", err)
			}

			contractBytes, err := content.FS.ReadFile(workflowContractEmbedPath)
			if err != nil {
				return fmt.Errorf("read embedded workflow contract: %w", err)
			}
			jsBytes, err := templates.FS.ReadFile(workflowJSEmbedPath)
			if err != nil {
				return fmt.Errorf("read embedded workflow js template: %w", err)
			}

			layers := []promptlayer.Layer{
				{
					ID:        "workflow-contract",
					Kind:      promptlayer.KindStable,
					Group:     promptlayer.GroupMethodologyTools,
					SourceRef: workflowContractDisplayPath,
					Content:   string(contractBytes),
				},
				{
					ID:        "workflow-schema",
					Kind:      promptlayer.KindStable,
					Group:     promptlayer.GroupMethodologyTools,
					SourceRef: workflowSchemaDisplayPath,
					Content:   string(schemaBytes),
				},
			}

			report := workflow.Render(schema, layers, string(jsBytes),
				workflowContractDisplayPath, workflowSchemaDisplayPath)

			printRenderReport(cmd, report, dryRun)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Render for inspection without executing any agent")
	return cmd
}

func printRenderReport(cmd *cobra.Command, report workflow.DryRunReport, dryRun bool) {
	out := cmd.OutOrStdout()
	mode := "render"
	if dryRun {
		mode = "render --dry-run"
	}
	fmt.Fprintf(out, "workflow %s\n", mode)
	fmt.Fprintf(out, "phase order: %s\n", strings.Join(report.PhaseOrder, ", "))
	fmt.Fprintf(out, "gate_build_test.verdict_source: %s\n", report.GateVerdictSource)
	fmt.Fprintf(out, "manifest: %s\n", report.ManifestPath)
	fmt.Fprintf(out, "schema: %s\n", report.SchemaPath)
	fmt.Fprintf(out, "prompt-manifest hash: %s\n", report.PromptManifestHash)
	fmt.Fprintln(out, "--- generated workflow js ---")
	fmt.Fprintln(out, report.JS)
}

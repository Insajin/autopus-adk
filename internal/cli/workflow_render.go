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

// newWorkflowRenderCmd loads the canonical manifest for the selected route from
// the embedded content/ and templates/ filesystems and emits the dry-run report
// (phase order, gate verdict source, manifest/schema paths, deterministic
// prompt-manifest hash, per-phase model/effort/depth, and the generated JS)
// without executing any agent (REQ-010/REQ-012, S7/S9/S11/S16/S18).
func newWorkflowRenderCmd() *cobra.Command {
	var dryRun bool
	var route string
	var quality string

	cmd := &cobra.Command{
		Use:           "render",
		Short:         "Render the generated workflow JS, manifest, schema, and prompt-manifest hash",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			re, routeKey, err := selectRouteEmbed(route)
			if err != nil {
				return err
			}

			schemaBytes, err := content.FS.ReadFile(re.schemaEmbed)
			if err != nil {
				return fmt.Errorf("read embedded workflow schema: %w", err)
			}
			schema, err := workflow.ParseSchema(schemaBytes)
			if err != nil {
				return fmt.Errorf("parse workflow schema: %w", err)
			}

			contractBytes, err := content.FS.ReadFile(re.contractEmbed)
			if err != nil {
				return fmt.Errorf("read embedded workflow contract: %w", err)
			}
			jsBytes, err := templates.FS.ReadFile(re.jsEmbed)
			if err != nil {
				return fmt.Errorf("read embedded workflow js template: %w", err)
			}

			layers := []promptlayer.Layer{
				{
					ID:        "workflow-contract",
					Kind:      promptlayer.KindStable,
					Group:     promptlayer.GroupMethodologyTools,
					SourceRef: re.contractDisplay,
					Content:   string(contractBytes),
				},
				{
					ID:        "workflow-schema",
					Kind:      promptlayer.KindStable,
					Group:     promptlayer.GroupMethodologyTools,
					SourceRef: re.schemaDisplay,
					Content:   string(schemaBytes),
				},
			}

			report := workflow.Render(schema, layers, string(jsBytes),
				re.contractDisplay, re.schemaDisplay)

			// Quality is an ephemeral overlay: it changes the per-phase model/
			// effort/depth view but never the prompt-manifest hash. Only the team
			// route carries a quality binding.
			if quality != "" && routeKey == "route_team" {
				binding := resolveTeamQualityBinding(quality, "")
				report.Phases = workflow.OverlayPhases(schema, &binding)
			}

			printRenderReport(cmd, report, dryRun)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Render for inspection without executing any agent")
	cmd.Flags().StringVar(&route, "route", "route_a", "Workflow route to render (route_a or route_team)")
	cmd.Flags().StringVar(&quality, "quality", "", "Quality tier overlay for the team route (ultra or balanced)")
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
	for _, ph := range report.Phases {
		fmt.Fprintf(out, "phase %s: model=%s effort=%s verify_votes=%d fan_out_cap=%d synthesis=%v\n",
			ph.ID, ph.Model, ph.Effort, ph.VerifyVotes, ph.FanOutCap, ph.Synthesis)
	}
	fmt.Fprintln(out, "--- generated workflow js ---")
	fmt.Fprintln(out, report.JS)
}

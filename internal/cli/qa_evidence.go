package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

type qaEvidencePayload struct {
	SchemaVersion         string                     `json:"schema_version"`
	QAResultID            string                     `json:"qa_result_id"`
	ManifestPath          string                     `json:"manifest_path"`
	Status                string                     `json:"status"`
	RedactionStatus       qaevidence.RedactionStatus `json:"redaction_status"`
	RepairPromptAvailable bool                       `json:"repair_prompt_available"`
	ArtifactKinds         []string                   `json:"artifact_kinds"`
}

func newQAEvidenceCmd() *cobra.Command {
	var (
		surface  string
		lane     string
		scenario string
		input    string
		output   string
		jsonOut  bool
		format   string
	)

	cmd := &cobra.Command{
		Use:   "evidence",
		Short: "Validate, redact, and write a QAMESH QA evidence manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAEvidence(cmd, qaEvidenceOptions{
				Surface:  surface,
				Lane:     lane,
				Scenario: scenario,
				Input:    input,
				Output:   output,
				JSONOut:  jsonOut,
				Format:   format,
			})
		},
	}
	cmd.Flags().StringVar(&surface, "surface", "", "Evidence surface (browser|desktop)")
	cmd.Flags().StringVar(&lane, "lane", "", "Evidence lane (fast|golden|native|release)")
	cmd.Flags().StringVar(&scenario, "scenario", "", "Scenario reference")
	cmd.Flags().StringVar(&input, "input", "", "Runner output or existing evidence manifest")
	cmd.Flags().StringVar(&output, "output", "", "Final evidence output directory")
	addJSONFlags(cmd, &jsonOut, &format)
	return cmd
}

type qaEvidenceOptions struct {
	Surface  string
	Lane     string
	Scenario string
	Input    string
	Output   string
	JSONOut  bool
	Format   string
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-001: CLI evidence command is the user-facing publish path for normalized QA manifests.
// @AX:REASON: Flag validation, manifest/source matching, artifact redaction, and JSON output envelope behavior are coupled at this boundary.
func runQAEvidence(cmd *cobra.Command, opts qaEvidenceOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	if err := validateQAEvidenceOptions(opts); err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_evidence_invalid_flags", map[string]any{"input": opts.Input, "output": opts.Output})
	}
	manifest, err := qaevidence.LoadManifest(opts.Input)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_evidence_read_failed", map[string]any{"input": opts.Input})
	}
	if err := validateManifestMatchesFlags(manifest, opts); err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_evidence_manifest_mismatch", map[string]any{"qa_result_id": manifest.QAResultID})
	}
	manifest, err = qaevidence.ResolveArtifactPaths(manifest, filepath.Dir(opts.Input))
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_evidence_artifact_path_invalid", map[string]any{"qa_result_id": manifest.QAResultID})
	}
	manifest = qaevidence.NormalizeManifest(manifest)
	manifestPath, err := qaevidence.WriteFinalManifest(manifest, opts.Output)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_evidence_write_failed", map[string]any{"qa_result_id": manifest.QAResultID})
	}
	payload := qaEvidencePayload{
		SchemaVersion:         manifest.SchemaVersion,
		QAResultID:            manifest.QAResultID,
		ManifestPath:          manifestPath,
		Status:                manifest.Status,
		RedactionStatus:       manifest.RedactionStatus,
		RepairPromptAvailable: manifest.Status == "failed",
		ArtifactKinds:         artifactKinds(manifest.Artifacts),
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, payload, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", manifest.QAResultID, manifestPath)
	return nil
}

func validateQAEvidenceOptions(opts qaEvidenceOptions) error {
	for name, value := range map[string]string{
		"surface":  opts.Surface,
		"lane":     opts.Lane,
		"scenario": opts.Scenario,
		"input":    opts.Input,
		"output":   opts.Output,
	} {
		if err := requireFlag(name, value); err != nil {
			return err
		}
	}
	if err := rejectGeneratedQAOutput("output", opts.Output); err != nil {
		return err
	}
	return nil
}

func validateManifestMatchesFlags(manifest qaevidence.Manifest, opts qaEvidenceOptions) error {
	if manifest.Surface != opts.Surface {
		return fmt.Errorf("manifest surface %q does not match --surface %q", manifest.Surface, opts.Surface)
	}
	if manifest.Lane != opts.Lane {
		return fmt.Errorf("manifest lane %q does not match --lane %q", manifest.Lane, opts.Lane)
	}
	if manifest.ScenarioRef != opts.Scenario {
		return fmt.Errorf("manifest scenario_ref %q does not match --scenario %q", manifest.ScenarioRef, opts.Scenario)
	}
	return nil
}

func artifactKinds(artifacts []qaevidence.ArtifactRef) []string {
	kinds := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		kinds = append(kinds, artifact.Kind)
	}
	return kinds
}

func qaCommandError(cmd *cobra.Command, jsonMode bool, err error, code string, data any) error {
	if jsonMode {
		return writeJSONResultAndExit(cmd, jsonStatusError, err, code, data, nil, nil)
	}
	return err
}

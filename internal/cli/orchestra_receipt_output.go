package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

const (
	orchestraOutputText      = "text"
	orchestraOutputJSON      = "json"
	orchestraCLIOutputSchema = "orchestration_cli_result.v1"
)

type orchestraCLIOutput struct {
	Schema  string                             `json:"schema"`
	Merged  string                             `json:"merged"`
	Receipt *orchestra.OrchestrationRunReceipt `json:"receipt"`
}

func validateOrchestraOutputFormat(format string) error {
	switch format {
	case "", orchestraOutputText, orchestraOutputJSON:
		return nil
	default:
		return fmt.Errorf("unsupported orchestra output format %q (use text or json)", format)
	}
}

func writeOrchestraCLIOutput(w io.Writer, result *orchestra.OrchestraResult, format string) error {
	if result == nil {
		return fmt.Errorf("orchestra output: result is required")
	}
	if format == "" {
		format = orchestraOutputText
	}
	if err := validateOrchestraOutputFormat(format); err != nil {
		return err
	}
	if format == orchestraOutputText {
		_, err := fmt.Fprintln(w, result.Merged)
		return err
	}
	if result.RunReceipt == nil || result.RunReceipt.Schema != orchestra.OrchestrationReceiptSchema {
		return fmt.Errorf("orchestra output: typed %s receipt is required", orchestra.OrchestrationReceiptSchema)
	}
	return json.NewEncoder(w).Encode(orchestraCLIOutput{
		Schema:  orchestraCLIOutputSchema,
		Merged:  result.Merged,
		Receipt: result.RunReceipt,
	})
}

func writeOrchestraReceiptArtifact(resultPath string, result *orchestra.OrchestraResult) (string, error) {
	if result == nil || result.RunReceipt == nil || result.RunReceipt.Schema != orchestra.OrchestrationReceiptSchema {
		return "", fmt.Errorf("orchestra receipt artifact: typed %s receipt is required", orchestra.OrchestrationReceiptSchema)
	}
	path := resultPath + ".receipt.json"
	data, err := json.MarshalIndent(result.RunReceipt, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal orchestra receipt: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write orchestra receipt: %w", err)
	}
	return path, nil
}

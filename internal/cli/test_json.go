package cli

import (
	"github.com/insajin/autopus-adk/pkg/e2e"
	"github.com/spf13/cobra"
)

type autoTestSummaryPayload struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Invalid int `json:"invalid"`
	Total   int `json:"total"`
}

type autoTestPayload struct {
	Summary     autoTestSummaryPayload        `json:"summary"`
	Results     []scenarioJSONResult          `json:"results"`
	Diagnostics []e2e.ScenarioValidationIssue `json:"diagnostics"`
}

func writeAutoTestJSON(
	cmd *cobra.Command,
	results []scenarioJSONResult,
	passed, failed, skipped int,
	cause error,
	warnings []jsonMessage,
) error {
	return writeAutoTestJSONWithDiagnostics(cmd, results, passed, failed, skipped, 0, nil, cause, warnings)
}

func writeAutoTestJSONWithDiagnostics(
	cmd *cobra.Command,
	results []scenarioJSONResult,
	passed, failed, skipped, invalid int,
	diagnostics []e2e.ScenarioValidationIssue,
	cause error,
	warnings []jsonMessage,
) error {
	if results == nil {
		results = []scenarioJSONResult{}
	}
	if diagnostics == nil {
		diagnostics = []e2e.ScenarioValidationIssue{}
	}
	payload := autoTestPayload{
		Summary: autoTestSummaryPayload{
			Passed:  passed,
			Failed:  failed,
			Skipped: skipped,
			Invalid: invalid,
			Total:   passed + failed + skipped,
		},
		Results:     results,
		Diagnostics: diagnostics,
	}

	status := jsonStatusOK
	if len(warnings) > 0 {
		status = jsonStatusWarn
	}

	if cause != nil {
		return writeJSONResultAndExit(cmd, status, cause, "test_run_failed", payload, warnings, nil)
	}
	return writeJSONResult(cmd, status, payload, warnings, nil)
}

func writeAutoTestValidationFailureJSON(
	cmd *cobra.Command,
	invalid int,
	diagnostics []e2e.ScenarioValidationIssue,
	cause error,
) error {
	payload := autoTestPayload{
		Summary:     autoTestSummaryPayload{Invalid: invalid},
		Results:     []scenarioJSONResult{},
		Diagnostics: diagnostics,
	}
	return writeJSONResultAndExit(
		cmd,
		jsonStatusError,
		cause,
		"test_scenarios_validation_failed",
		payload,
		nil,
		nil,
	)
}

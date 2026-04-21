package cli

import "github.com/spf13/cobra"

type autoTestSummaryPayload struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

type autoTestPayload struct {
	Summary autoTestSummaryPayload `json:"summary"`
	Results []scenarioJSONResult   `json:"results"`
}

func writeAutoTestJSON(
	cmd *cobra.Command,
	results []scenarioJSONResult,
	passed, failed, skipped int,
	cause error,
	warnings []jsonMessage,
) error {
	payload := autoTestPayload{
		Summary: autoTestSummaryPayload{
			Passed:  passed,
			Failed:  failed,
			Skipped: skipped,
			Total:   passed + failed + skipped,
		},
		Results: results,
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

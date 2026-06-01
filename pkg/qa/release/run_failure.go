package release

import qarun "github.com/insajin/autopus-adk/pkg/qa/run"

func firstRunFailure(result qarun.Result) (string, string) {
	for _, check := range result.Checks {
		if check.Status == "failed" || check.Status == "blocked" {
			return check.JourneyID, check.FailureSummary
		}
	}
	for _, adapterResult := range result.AdapterResults {
		if adapterResult.Status == "failed" || adapterResult.Status == "blocked" {
			return adapterResult.JourneyID, adapterResult.FailureSummary
		}
	}
	for _, failedCheck := range result.FailedChecks {
		if failedCheck != "" {
			return failedCheck, ""
		}
	}
	return "", ""
}

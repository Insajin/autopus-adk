package design

import "strings"

func playwrightExecutionCheck(playwrightErr string) VisualCheck {
	playwrightErr = strings.TrimSpace(playwrightErr)
	if playwrightErr == "" {
		return VisualCheck{ID: "playwright_execution", Status: "PASS", Severity: "info", Message: "Playwright execution completed"}
	}
	return VisualCheck{
		ID:       "playwright_execution",
		Status:   "FAIL",
		Severity: "high",
		Message:  "Playwright execution failed: " + playwrightErr,
	}
}

package cli

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func printChecklistSummary(outcomes []spec.ChecklistOutcome) {
	if len(outcomes) == 0 {
		return
	}

	passCount, failCount, naCount := spec.CountChecklistStatuses(outcomes)

	fmt.Printf("체크리스트 결과: %d건 (PASS: %d, FAIL: %d, N/A: %d)\n",
		len(outcomes), passCount, failCount, naCount)
	for _, outcome := range outcomes {
		if outcome.Status != spec.ChecklistStatusFail {
			continue
		}
		if outcome.Reason == "" {
			fmt.Printf("- [FAIL] %s\n", outcome.ID)
			continue
		}
		fmt.Printf("- [FAIL] %s: %s\n", outcome.ID, outcome.Reason)
	}
}

package cli

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func printChecklistSummary(outcomes []spec.ChecklistOutcome) {
	if len(outcomes) == 0 {
		return
	}

	passCount := 0
	failCount := 0
	for _, outcome := range outcomes {
		switch outcome.Status {
		case spec.ChecklistStatusPass:
			passCount++
		case spec.ChecklistStatusFail:
			failCount++
		}
	}

	fmt.Printf("체크리스트 결과: %d건 (PASS: %d, FAIL: %d)\n", len(outcomes), passCount, failCount)
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

package experiment

import (
	"math"
	"sort"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

type pairedTrial struct {
	identity ComparisonIdentity
	order    string
	raw      int64
	valid    bool
	reason   string
}

type taskArms struct {
	baseline  *pairedTrial
	candidate *pairedTrial
}

// CompareCompatibleTasks pairs only accepted baseline/candidate tasks in the
// exact same comparison stratum. Retries and failed attempts remain in spend.
func CompareCompatibleTasks(trials []TaskTrial) PairedComparison {
	return compareTasks(trials, nil, false)
}

// CompareExpectedTasks reconciles every expected task against strict paired
// evidence. Unexpected trials are retained but never affect paired arithmetic.
func CompareExpectedTasks(trials []TaskTrial, expectedTaskIDs []string) PairedComparison {
	return compareTasks(trials, expectedTaskIDs, true)
}

func compareTasks(trials []TaskTrial, expectedTaskIDs []string, strict bool) PairedComparison {
	grouped := make(map[string]*taskArms)
	expected := make(map[string]struct{}, len(expectedTaskIDs))
	unexpected := make(map[string]struct{})
	if strict {
		for _, taskID := range expectedTaskIDs {
			if taskID == "" {
				continue
			}
			expected[taskID] = struct{}{}
			grouped[taskID] = &taskArms{}
		}
	}
	for _, trial := range trials {
		if trial.TaskID == "" {
			continue
		}
		if strict {
			if _, ok := expected[trial.TaskID]; !ok {
				unexpected[trial.TaskID] = struct{}{}
				continue
			}
		}
		summary := telemetry.SummarizeEfficiency(trial.Runs)
		accepted := summary.AcceptedTasks == 1 && !summary.PromotionBlocked && summary.RawTotalTokensPerAcceptedTask != nil
		if !accepted && !strict {
			continue
		}
		entry := &pairedTrial{identity: trial.Identity, order: trial.PairOrder, raw: summary.RawTokens, valid: accepted && trialUsageMatchesIdentity(trial)}
		if !accepted {
			entry.reason = "unaccepted_trial"
		}
		if !completeComparisonIdentity(trial.Identity) || trial.PairOrder != "AB" && trial.PairOrder != "BA" {
			entry.valid = false
			entry.reason = "incompatible_stratum"
		}
		arms := grouped[trial.TaskID]
		if arms == nil {
			arms = &taskArms{}
			grouped[trial.TaskID] = arms
		}
		switch trial.Arm {
		case "baseline":
			if arms.baseline != nil {
				entry.valid = false
				if strict {
					entry.reason = "duplicate_arm"
				}
			}
			arms.baseline = entry
		case "candidate":
			if arms.candidate != nil {
				entry.valid = false
				if strict {
					entry.reason = "duplicate_arm"
				}
			}
			arms.candidate = entry
		}
	}

	result := PairedComparison{Provisional25PctTarget: "NOT_MET"}
	if strict {
		result.ExpectedTaskIDs = mapKeys(expected)
		result.ExpectedTaskCount = len(result.ExpectedTaskIDs)
		result.UnexpectedTaskIDs = mapKeys(unexpected)
		for _, taskID := range result.UnexpectedTaskIDs {
			result.ExcludedTasks = append(result.ExcludedTasks, ExcludedTask{TaskID: taskID, Reason: "unexpected_task"})
		}
	}
	reductions := make([]float64, 0, len(grouped))
	for taskID, arms := range grouped {
		if arms.baseline == nil || arms.candidate == nil {
			result.UnpairedTaskIDs = append(result.UnpairedTaskIDs, taskID)
			continue
		}
		if !arms.baseline.valid || !arms.candidate.valid ||
			arms.baseline.identity != arms.candidate.identity ||
			arms.baseline.order != arms.candidate.order {
			result.ExcludedTasks = append(result.ExcludedTasks, ExcludedTask{TaskID: taskID, Reason: excludedTaskReason(arms)})
			continue
		}
		result.PairedTaskIDs = append(result.PairedTaskIDs, taskID)
		result.PairedARawTokens += arms.baseline.raw
		result.PairedBRawTokens += arms.candidate.raw
		if arms.baseline.order == "AB" {
			result.ABTaskIDs = append(result.ABTaskIDs, taskID)
		} else if arms.baseline.order == "BA" {
			result.BATaskIDs = append(result.BATaskIDs, taskID)
		}
		if arms.baseline.raw > 0 {
			reductions = append(reductions, reductionPct(arms.baseline.raw, arms.candidate.raw))
		}
	}

	sort.Strings(result.PairedTaskIDs)
	sort.Strings(result.UnpairedTaskIDs)
	sort.Strings(result.ABTaskIDs)
	sort.Strings(result.BATaskIDs)
	sort.Slice(result.ExcludedTasks, func(i, j int) bool { return result.ExcludedTasks[i].TaskID < result.ExcludedTasks[j].TaskID })
	result.PairedTaskCount = len(result.PairedTaskIDs)
	if strict {
		result.PairedExpectedTaskCount = result.PairedTaskCount
		result.ExpectedCorpusComplete = result.ExpectedTaskCount > 0 &&
			result.PairedExpectedTaskCount == result.ExpectedTaskCount && len(result.UnexpectedTaskIDs) == 0
	}
	if result.PairedARawTokens > 0 {
		result.PairedReductionPct = reductionPct(result.PairedARawTokens, result.PairedBRawTokens)
	}
	result.MedianPairedRawReductionPct = median(reductions)
	if result.PairedTaskCount > 0 && result.MedianPairedRawReductionPct >= 25 {
		result.Provisional25PctTarget = "PASS"
	}
	return result
}

func excludedTaskReason(arms *taskArms) string {
	for _, reason := range []string{"duplicate_arm", "unaccepted_trial"} {
		if arms.baseline.reason == reason || arms.candidate.reason == reason {
			return reason
		}
	}
	return "incompatible_stratum"
}

func mapKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func trialUsageMatchesIdentity(trial TaskTrial) bool {
	found := false
	for _, run := range trial.Runs {
		for _, usage := range run.Usage {
			found = true
			if !usageMatchesComparisonIdentity(usage, trial.Identity) {
				return false
			}
		}
	}
	return found
}

func reductionPct(before, after int64) float64 {
	return math.Round(((float64(before-after)/float64(before))*100)*1000) / 1000
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}

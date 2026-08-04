package promptlayer

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

func validateOMPContextPromotionCohortV1(report OMPContextPromotionReportV1) error {
	if len(report.Tasks) != 20 || len(report.Observations) != 40 {
		return fmt.Errorf("OMP context promotion cohort must contain exactly 20 tasks and 40 observations")
	}
	rows, abCount, baCount, err := validateOMPContextPromotionObservationsV1(report)
	if err != nil {
		return err
	}
	aggregate, err := ReduceOMPContextCanaryPairsV1(rows)
	if err != nil || aggregate.PairCount != 20 || aggregate.ABCount != 10 || aggregate.BACount != 10 ||
		abCount != 10 || baCount != 10 || aggregate.IntegrityFailures != 0 || aggregate.SecurityFailures != 0 ||
		aggregate.QualityRegressions != 0 || !aggregate.FallbackVerified || !aggregate.RollbackVerified ||
		aggregate.MedianReductionBasisPoints < report.Policy.MinReductionBasisPoints {
		return fmt.Errorf("OMP context promotion cohort gates failed")
	}
	expectedGates := expectedOMPContextPromotionGatesV1(aggregate.MedianReductionBasisPoints)
	if !reflect.DeepEqual(report.Gates, expectedGates) {
		return fmt.Errorf("OMP context promotion gate projection mismatch")
	}
	return nil
}

func validateOMPContextPromotionObservationsV1(report OMPContextPromotionReportV1) ([]OMPContextCanaryRowV1, int, int, error) {
	tasks := make(map[string]bool, 20)
	observationDigests := make(map[string]bool, 40)
	usageDigests := make(map[string]bool, 40)
	rows := make([]OMPContextCanaryRowV1, 0, 40)
	var previousCompleted time.Time
	abCount, baCount := 0, 0
	for taskIndex, task := range report.Tasks {
		if !validOMPContextMemoryHashV1(task.TaskIDDigest) || tasks[task.TaskIDDigest] || (task.Order != "AB" && task.Order != "BA") {
			return nil, 0, 0, fmt.Errorf("OMP context promotion task is invalid or duplicate")
		}
		tasks[task.TaskIDDigest] = true
		if task.Order == "AB" {
			abCount++
		} else {
			baCount++
		}
		for pairIndex := 0; pairIndex < 2; pairIndex++ {
			observation := report.Observations[taskIndex*2+pairIndex]
			expectedVariant := string(task.Order[pairIndex])
			if err := validateOMPContextPromotionObservationV1(report, observation, task.TaskIDDigest,
				expectedVariant, taskIndex*2+pairIndex+1, previousCompleted, observationDigests, usageDigests); err != nil {
				return nil, 0, 0, err
			}
			completed, _ := time.Parse(time.RFC3339Nano, observation.CompletedAt)
			previousCompleted = completed
			rows = append(rows, OMPContextCanaryRowV1{
				TaskID: observation.TaskIDDigest, Variant: OMPContextCanaryVariantV1(observation.Variant), Order: pairIndex + 1,
				Tokens: observation.InputTokens, IntegrityPassed: observation.IntegrityPassed,
				SecurityPassed: observation.SecurityPassed, QualityScore: observation.QualityScore,
				FallbackVerified: observation.FallbackVerified, RollbackVerified: observation.RollbackVerified,
			})
		}
	}
	return rows, abCount, baCount, nil
}

func validateOMPContextPromotionObservationV1(report OMPContextPromotionReportV1, observation OMPContextPromotionObservationV1,
	taskDigest, variant string, sequence int, previousCompleted time.Time, observationDigests, usageDigests map[string]bool) error {
	started, startErr := parseCanonicalOMPContextPromotionTimeV2(observation.StartedAt)
	completed, completeErr := parseCanonicalOMPContextPromotionTimeV2(observation.CompletedAt)
	if startErr != nil || completeErr != nil || !started.Before(completed) || (!previousCompleted.IsZero() && started.Before(previousCompleted)) {
		return fmt.Errorf("OMP context promotion observations are not strictly serial")
	}
	if observation.Sequence != sequence || observation.TaskIDDigest != taskDigest || observation.Variant != variant ||
		observation.Provider != report.Provider || observation.ModelScopeDigest != report.ModelScopeDigest ||
		observation.EndpointClass != "external-provider" || observation.Transport != "provider-api" ||
		observation.CredentialMode != "locator-only" || observation.ExecutionMode != "external-live" {
		return fmt.Errorf("OMP context promotion observation binding is invalid")
	}
	if observation.InputTokens <= 0 || observation.OutputTokens <= 0 || observation.TotalTokens <= 0 ||
		observation.InputTokens > 1_000_000_000_000 || observation.OutputTokens > 1_000_000_000_000 ||
		observation.TotalTokens != observation.InputTokens+observation.OutputTokens || observation.QualityScore <= 0 ||
		observation.QualityScore > 1_000_000_000 || observation.RetryCount != 0 || observation.MaxConcurrency != 1 {
		return fmt.Errorf("OMP context promotion observation facts are invalid")
	}
	if !observation.IntegrityPassed || !observation.SecurityPassed || !observation.FallbackVerified ||
		!observation.RollbackVerified || !observation.CleanupVerified || !validOMPContextMemoryHashV1(observation.ObservationDigest) ||
		!validOMPContextMemoryHashV1(observation.UsageDigest) || observationDigests[observation.ObservationDigest] ||
		usageDigests[observation.UsageDigest] {
		return fmt.Errorf("OMP context promotion observation proof is invalid or duplicate")
	}
	observationDigests[observation.ObservationDigest] = true
	usageDigests[observation.UsageDigest] = true
	return nil
}

func expectedOMPContextPromotionGatesV1(medianBasisPoints int64) []OMPContextPromotionGateResultV1 {
	pass := func(id, observed, required string) OMPContextPromotionGateResultV1 {
		return OMPContextPromotionGateResultV1{GateID: id, Status: "passed", ObservedValue: observed, RequiredValue: required, Reason: "gate-passed"}
	}
	return []OMPContextPromotionGateResultV1{
		pass("integrity", "40/40", "40/40"), pass("security", "40/40", "40/40"), pass("quality", "0", "0"),
		pass("pair-count", "20", "20"), pass("balanced-order", "10:10", "10:10"),
		pass("token-reduction", strconv.FormatInt(medianBasisPoints, 10), "2000"),
		pass("fallback", "40/40", "40/40"), pass("rollback", "40/40", "40/40"), pass("cleanup", "40/40", "40/40"),
		pass("serial-execution", "40/40", "40/40"), pass("no-retry", "0", "0"),
	}
}

func parseCanonicalOMPContextPromotionTimeV2(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.UTC().Format(time.RFC3339Nano) != value {
		return time.Time{}, fmt.Errorf("OMP context promotion timestamp is invalid")
	}
	return parsed, nil
}

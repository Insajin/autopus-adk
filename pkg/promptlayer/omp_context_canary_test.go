package promptlayer

import (
	"testing"
)

func TestReduceOMPContextCanaryPairsV1_ABBAFormulaAndMedian(t *testing.T) {
	t.Parallel()
	rows := []OMPContextCanaryRowV1{
		{TaskID: "T2", Variant: OMPContextCanaryVariantOptimizedV1, Order: 1, Tokens: 6000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100, FallbackVerified: true, RollbackVerified: true},
		{TaskID: "T1", Variant: OMPContextCanaryVariantFullV1, Order: 1, Tokens: 10000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100},
		{TaskID: "T2", Variant: OMPContextCanaryVariantFullV1, Order: 2, Tokens: 10000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100},
		{TaskID: "T1", Variant: OMPContextCanaryVariantOptimizedV1, Order: 2, Tokens: 7000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100, FallbackVerified: true, RollbackVerified: true},
	}

	result, err := ReduceOMPContextCanaryPairsV1(rows)
	if err != nil {
		t.Fatalf("reduce canary pairs: %v", err)
	}
	if len(result.PairKeys) != 2 || result.PairKeys[0] != "T1" || result.PairKeys[1] != "T2" {
		t.Fatalf("pair keys = %v", result.PairKeys)
	}
	if result.Pairs[0].ReductionPercent != "30.00" || result.Pairs[1].ReductionPercent != "40.00" || result.MedianReductionPercent != "35.00" {
		t.Fatalf("reductions = %+v median=%s", result.Pairs, result.MedianReductionPercent)
	}
	if result.ABCount != 1 || result.BACount != 1 || !result.BalancedOrder {
		t.Fatalf("order aggregation mismatch: %+v", result)
	}
	if result.UnpairedRows != 0 || result.DuplicateRows != 0 || result.IntegrityFailures != 0 || result.SecurityFailures != 0 || result.QualityRegressions != 0 {
		t.Fatalf("unexpected invalid canary evidence: %+v", result)
	}
}

func TestReduceOMPContextCanaryPairsV1_IsDeterministicAcrossInputOrder(t *testing.T) {
	t.Parallel()
	base := []OMPContextCanaryRowV1{
		{TaskID: "A", Variant: OMPContextCanaryVariantFullV1, Order: 1, Tokens: 100, IntegrityPassed: true, SecurityPassed: true, QualityScore: 10},
		{TaskID: "A", Variant: OMPContextCanaryVariantOptimizedV1, Order: 2, Tokens: 80, IntegrityPassed: true, SecurityPassed: true, QualityScore: 10, FallbackVerified: true, RollbackVerified: true},
		{TaskID: "B", Variant: OMPContextCanaryVariantOptimizedV1, Order: 1, Tokens: 70, IntegrityPassed: true, SecurityPassed: true, QualityScore: 10, FallbackVerified: true, RollbackVerified: true},
		{TaskID: "B", Variant: OMPContextCanaryVariantFullV1, Order: 2, Tokens: 100, IntegrityPassed: true, SecurityPassed: true, QualityScore: 10},
	}
	first, err := ReduceOMPContextCanaryPairsV1(base)
	if err != nil {
		t.Fatal(err)
	}
	reversed := append([]OMPContextCanaryRowV1(nil), base...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	second, err := ReduceOMPContextCanaryPairsV1(reversed)
	if err != nil {
		t.Fatal(err)
	}
	if !equalOMPContextCanaryAggregate(first, second) {
		t.Fatalf("aggregate changed with insertion order\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestReduceOMPContextCanaryPairsV1_RejectsDuplicateUnpairedAndInvalidRows(t *testing.T) {
	t.Parallel()
	rows := []OMPContextCanaryRowV1{
		{TaskID: "duplicate", Variant: OMPContextCanaryVariantFullV1, Order: 1, Tokens: 100},
		{TaskID: "duplicate", Variant: OMPContextCanaryVariantFullV1, Order: 2, Tokens: 90},
		{TaskID: "unpaired", Variant: OMPContextCanaryVariantOptimizedV1, Order: 1, Tokens: 80},
		{TaskID: "invalid", Variant: "other", Order: 1, Tokens: 0},
	}
	result, err := ReduceOMPContextCanaryPairsV1(rows)
	if err == nil {
		t.Fatal("expected malformed pair error")
	}
	if result.DuplicateRows == 0 || result.UnpairedRows == 0 || result.InvalidRows == 0 {
		t.Fatalf("invalid row counters missing: %+v", result)
	}
}

func TestReduceOMPContextCanaryPairsV1_NegativeAndOddMedianAreDeterministic(t *testing.T) {
	t.Parallel()
	rows := []OMPContextCanaryRowV1{
		{TaskID: "A", Variant: OMPContextCanaryVariantFullV1, Order: 1, Tokens: 100, IntegrityPassed: true, SecurityPassed: true},
		{TaskID: "A", Variant: OMPContextCanaryVariantOptimizedV1, Order: 2, Tokens: 110, IntegrityPassed: true, SecurityPassed: true, FallbackVerified: true, RollbackVerified: true},
		{TaskID: "B", Variant: OMPContextCanaryVariantOptimizedV1, Order: 1, Tokens: 80, IntegrityPassed: true, SecurityPassed: true, FallbackVerified: true, RollbackVerified: true},
		{TaskID: "B", Variant: OMPContextCanaryVariantFullV1, Order: 2, Tokens: 100, IntegrityPassed: true, SecurityPassed: true},
		{TaskID: "C", Variant: OMPContextCanaryVariantFullV1, Order: 1, Tokens: 100, IntegrityPassed: true, SecurityPassed: true},
		{TaskID: "C", Variant: OMPContextCanaryVariantOptimizedV1, Order: 2, Tokens: 70, IntegrityPassed: true, SecurityPassed: true, FallbackVerified: true, RollbackVerified: true},
	}
	result, err := ReduceOMPContextCanaryPairsV1(rows)
	if err != nil {
		t.Fatal(err)
	}
	if result.Pairs[0].ReductionPercent != "-10.00" || result.MedianReductionPercent != "20.00" {
		t.Fatalf("negative/odd median mismatch: %+v", result)
	}
}

func equalOMPContextCanaryAggregate(a, b OMPContextCanaryAggregateV1) bool {
	if a.PairCount != b.PairCount || a.MedianReductionBasisPoints != b.MedianReductionBasisPoints || a.ABCount != b.ABCount || a.BACount != b.BACount {
		return false
	}
	if len(a.Pairs) != len(b.Pairs) {
		return false
	}
	for index := range a.Pairs {
		if a.Pairs[index] != b.Pairs[index] {
			return false
		}
	}
	return true
}

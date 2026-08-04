package promptlayer

import (
	"fmt"
	"sort"
	"strconv"
)

type ompContextCanaryGroupV1 struct {
	full      []OMPContextCanaryRowV1
	optimized []OMPContextCanaryRowV1
}

// @AX:WARN [AUTO]: canary pair reduction has cyclomatic complexity 26.
// @AX:REASON [AUTO]: gocyclo reports 26 across row validation, pair reconciliation, metric aggregation, and semantic-equivalence checks.
func ReduceOMPContextCanaryPairsV1(rows []OMPContextCanaryRowV1) (OMPContextCanaryAggregateV1, error) {
	result := OMPContextCanaryAggregateV1{
		SchemaVersion: OMPContextCanarySchemaV1, PairKeys: []string{}, Pairs: []OMPContextCanaryPairV1{},
		FallbackVerified: true, RollbackVerified: true,
	}
	groups := make(map[string]*ompContextCanaryGroupV1)
	for _, row := range rows {
		if !validOMPContextCanaryRowV1(row) {
			result.InvalidRows++
			continue
		}
		group := groups[row.TaskID]
		if group == nil {
			group = &ompContextCanaryGroupV1{}
			groups[row.TaskID] = group
		}
		switch row.Variant {
		case OMPContextCanaryVariantFullV1:
			group.full = append(group.full, row)
		case OMPContextCanaryVariantOptimizedV1:
			group.optimized = append(group.optimized, row)
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	reductions := make([]int64, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		if len(group.full) > 1 {
			result.DuplicateRows += len(group.full) - 1
		}
		if len(group.optimized) > 1 {
			result.DuplicateRows += len(group.optimized) - 1
		}
		if len(group.full) != 1 || len(group.optimized) != 1 {
			result.UnpairedRows += len(group.full) + len(group.optimized)
			continue
		}
		full, optimized := group.full[0], group.optimized[0]
		if full.Order == optimized.Order {
			result.InvalidRows += 2
			continue
		}
		reduction := ompContextReductionBasisPointsV1(full.Tokens, optimized.Tokens)
		order := "AB"
		if optimized.Order < full.Order {
			order = "BA"
			result.BACount++
		} else {
			result.ABCount++
		}
		pair := OMPContextCanaryPairV1{
			TaskID: key, Order: order, FullTokens: full.Tokens, OptimizedTokens: optimized.Tokens,
			ReductionBasisPoints: reduction, ReductionPercent: ompContextPercentV1(reduction),
			IntegrityPassed:  full.IntegrityPassed && optimized.IntegrityPassed,
			SecurityPassed:   full.SecurityPassed && optimized.SecurityPassed,
			QualityDelta:     optimized.QualityScore - full.QualityScore,
			FallbackVerified: optimized.FallbackVerified, RollbackVerified: optimized.RollbackVerified,
		}
		if !pair.IntegrityPassed {
			result.IntegrityFailures++
		}
		if !pair.SecurityPassed {
			result.SecurityFailures++
		}
		if pair.QualityDelta < 0 {
			result.QualityRegressions++
		}
		result.FallbackVerified = result.FallbackVerified && pair.FallbackVerified
		result.RollbackVerified = result.RollbackVerified && pair.RollbackVerified
		result.PairKeys = append(result.PairKeys, key)
		result.Pairs = append(result.Pairs, pair)
		reductions = append(reductions, reduction)
	}
	result.PairCount = len(result.Pairs)
	result.BalancedOrder = result.PairCount > 0 && absOMPContextCanaryIntV1(result.ABCount-result.BACount) <= 1
	result.MedianReductionBasisPoints = medianOMPContextCanaryV1(reductions)
	result.MedianReductionPercent = ompContextPercentV1(result.MedianReductionBasisPoints)
	if result.PairCount == 0 {
		result.FallbackVerified, result.RollbackVerified = false, false
	}
	if result.InvalidRows > 0 || result.DuplicateRows > 0 || result.UnpairedRows > 0 {
		return result, fmt.Errorf("OMP context canary rows are not exact pairs: invalid=%d duplicate=%d unpaired=%d", result.InvalidRows, result.DuplicateRows, result.UnpairedRows)
	}
	return result, nil
}

func validOMPContextCanaryRowV1(row OMPContextCanaryRowV1) bool {
	return safeOMPContextMemoryMetadataV1(row.TaskID) &&
		(row.Variant == OMPContextCanaryVariantFullV1 || row.Variant == OMPContextCanaryVariantOptimizedV1) &&
		row.Order > 0 && row.Order <= 2 && row.Tokens > 0 && row.Tokens <= 1_000_000_000_000 &&
		row.QualityScore >= -1_000_000_000 && row.QualityScore <= 1_000_000_000
}

func ompContextReductionBasisPointsV1(full, optimized int64) int64 {
	numerator := (full - optimized) * 10_000
	if numerator >= 0 {
		return (numerator + full/2) / full
	}
	return (numerator - full/2) / full
}

func medianOMPContextCanaryV1(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]int64(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}

func ompContextPercentV1(basisPoints int64) string {
	sign := ""
	if basisPoints < 0 {
		sign, basisPoints = "-", -basisPoints
	}
	return sign + strconv.FormatInt(basisPoints/100, 10) + "." + fmt.Sprintf("%02d", basisPoints%100)
}

func absOMPContextCanaryIntV1(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

package promptlayer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func ompContextPromotionProviderAuthorityDigestV1(report OMPContextPromotionReportV1) string {
	if len(report.Observations) != ompContextPromotionTaskCountV1*2 {
		return ""
	}
	digest := report.Observations[0].ProviderAuthorityDigest
	for _, observation := range report.Observations[1:] {
		if observation.ProviderAuthorityDigest != digest {
			return ""
		}
	}
	return digest
}

func ompContextPromotionSessionAuthorityDigestV1(report OMPContextPromotionReportV1) string {
	receipts := map[string][]string{
		"A": make([]string, ompContextPromotionSessionCountV1),
		"B": make([]string, ompContextPromotionSessionCountV1),
	}
	variantSequences := map[string]int{"A": 0, "B": 0}
	allReceipts := make(map[string]bool, ompContextPromotionSessionCountV1*2)
	for _, observation := range report.Observations {
		variantReceipts, ok := receipts[observation.Variant]
		if !ok {
			return ""
		}
		variantSequences[observation.Variant]++
		variantSequence := variantSequences[observation.Variant]
		segment := (variantSequence - 1) / ompContextPromotionSessionSegmentSizeV1
		sessionSequence := (variantSequence-1)%ompContextPromotionSessionSegmentSizeV1 + 1
		if segment >= len(variantReceipts) || observation.SessionSequence != sessionSequence {
			return ""
		}
		if variantReceipts[segment] == "" {
			if allReceipts[observation.SessionReceiptDigest] {
				return ""
			}
			variantReceipts[segment] = observation.SessionReceiptDigest
			allReceipts[observation.SessionReceiptDigest] = true
		} else if variantReceipts[segment] != observation.SessionReceiptDigest {
			return ""
		}
	}
	providerAuthority := ompContextPromotionProviderAuthorityDigestV1(report)
	if variantSequences["A"] != ompContextPromotionTaskCountV1 ||
		variantSequences["B"] != ompContextPromotionTaskCountV1 ||
		len(allReceipts) != ompContextPromotionSessionCountV1*2 ||
		!validOMPContextMemoryHashV1(providerAuthority) {
		return ""
	}
	body, err := json.Marshal(struct {
		Producer          OMPContextPromotionProducerV1     `json:"producer"`
		SessionFacts      OMPContextPromotionSessionFactsV1 `json:"session_facts"`
		Provider          string                            `json:"provider"`
		ModelScopeDigest  string                            `json:"model_scope_digest"`
		ProviderAuthority string                            `json:"provider_authority_digest"`
		FullSessions      []string                          `json:"full_session_receipt_digests"`
		OptimizedSessions []string                          `json:"optimized_session_receipt_digests"`
		CohortDigest      string                            `json:"cohort_manifest_digest"`
	}{
		report.Producer, report.SessionFacts, report.Provider, report.ModelScopeDigest, providerAuthority,
		receipts["A"], receipts["B"], report.CohortManifestDigest,
	})
	if err != nil {
		return ""
	}
	return promotionSHA256(body)
}

func expectedOMPContextPromotionGatesV1(
	medianBasisPoints int64,
	compactionCount int,
) []OMPContextPromotionGateResultV1 {
	pass := func(id, observed, required string) OMPContextPromotionGateResultV1 {
		return OMPContextPromotionGateResultV1{GateID: id, Status: "passed", ObservedValue: observed, RequiredValue: required, Reason: "gate-passed"}
	}
	return []OMPContextPromotionGateResultV1{
		pass("integrity", "40/40", "40/40"), pass("security", "40/40", "40/40"), pass("quality", "0", "0"),
		pass("pair-count", "20", "20"), pass("balanced-order", "10:10", "10:10"),
		pass("provider-authority", "40/40", "40/40"), pass("reusable-session", "38/38", "38/38"),
		pass("multi-compaction-admission", strconv.Itoa(compactionCount)+"/20", ">=2/20"),
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

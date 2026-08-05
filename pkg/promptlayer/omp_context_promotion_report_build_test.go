package promptlayer

import (
	"bytes"
	"testing"
)

func TestBuildOMPContextPromotionReportV1_DerivesCanonicalBodyFreeIdentity(t *testing.T) {
	report := validOMPContextPromotionReportV1()
	report.EvidenceID = ""
	report.TrustLane = ""

	built, body, err := BuildOMPContextPromotionReportV1(report)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeOMPContextPromotionReportV1(body)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.EvidenceID != built.EvidenceID || decoded.TrustLane != OMPContextPromotionTrustLaneV2 {
		t.Fatalf("canonical identity mismatch: %#v", decoded)
	}
	for _, forbidden := range [][]byte{[]byte("prompt_body"), []byte("assistant_text"), []byte("credential_locator")} {
		if bytes.Contains(body, forbidden) {
			t.Fatalf("body-bearing field leaked: %q", forbidden)
		}
	}
	if got := ompContextPromotionProviderAuthorityDigestV1(built); got == "" {
		t.Fatal("provider authority digest missing")
	}
	if got := ompContextPromotionSessionAuthorityDigestV1(built); got == "" {
		t.Fatal("session authority digest missing")
	}
	rows, err := OMPContextPromotionReportCanaryRowsV1(built)
	if err != nil || len(rows) != 40 {
		t.Fatalf("canary projection failed: rows=%d err=%v", len(rows), err)
	}
}

func TestBuildOMPContextPromotionReportV1_RejectsIncompleteObservedCohort(t *testing.T) {
	report := validOMPContextPromotionReportV1()
	report.Observations = report.Observations[:39]
	if _, _, err := BuildOMPContextPromotionReportV1(report); err == nil {
		t.Fatal("incomplete cohort accepted")
	}
}

func TestBuildOMPContextPromotionReportV1_RejectsSessionOrProviderAuthorityDrift(t *testing.T) {
	for name, mutate := range map[string]func(*OMPContextPromotionReportV1){
		"session": func(report *OMPContextPromotionReportV1) {
			report.Observations[2].SessionReceiptDigest = promotionSHA256([]byte("different-session"))
		},
		"provider": func(report *OMPContextPromotionReportV1) {
			report.Observations[2].ProviderAuthorityDigest = promotionSHA256([]byte("different-provider-authority"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			report := validOMPContextPromotionReportV1()
			mutate(&report)
			if _, _, err := BuildOMPContextPromotionReportV1(report); err == nil {
				t.Fatal("authority drift accepted")
			}
		})
	}
}

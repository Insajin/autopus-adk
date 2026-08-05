package promptlayer

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestWriteOMPContextPromotionReportV1_WritesCanonicalBytesAtCanonicalPath(t *testing.T) {
	report := validOMPContextPromotionReportV1()
	report.EvidenceID = ""
	report.TrustLane = ""
	_, body, err := BuildOMPContextPromotionReportV1(report)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	wantPath := filepath.Join(root, ".autopus", "runtime", "omp-context", "promotion-report-v1.json")
	if got := OMPContextPromotionReportPathV1(root); got != wantPath {
		t.Fatalf("report path = %q, want %q", got, wantPath)
	}

	if err := WriteOMPContextPromotionReportV1(root, body); err != nil {
		t.Fatalf("write promotion report: %v", err)
	}
	written, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, body) {
		t.Fatal("persisted promotion report bytes differ from canonical input")
	}
	info, err := os.Lstat(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("promotion report mode = %v", info.Mode())
	}

	nonCanonical := append(append([]byte(nil), body...), '\n')
	if err := WriteOMPContextPromotionReportV1(root, nonCanonical); err == nil {
		t.Fatal("non-canonical replacement was accepted")
	}
	preserved, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(preserved, body) {
		t.Fatal("rejected replacement changed canonical promotion report")
	}
}

func TestWriteOMPContextPromotionReportV1_RejectsInvalidBodyWithoutWriting(t *testing.T) {
	report := validOMPContextPromotionReportV1()
	report.EvidenceID = ""
	report.TrustLane = ""
	_, canonical, err := BuildOMPContextPromotionReportV1(report)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		body []byte
	}{
		{name: "empty", body: nil},
		{name: "malformed JSON", body: []byte("{")},
		{name: "non-canonical JSON", body: append(append([]byte(nil), canonical...), '\n')},
		{name: "oversized", body: make([]byte, ompContextPromotionReportMaxBytesV1+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := WriteOMPContextPromotionReportV1(root, test.body); err == nil {
				t.Fatal("invalid promotion report was written")
			}
			if _, err := os.Lstat(OMPContextPromotionReportPathV1(root)); !os.IsNotExist(err) {
				t.Fatalf("invalid report created canonical artifact: %v", err)
			}
		})
	}
}

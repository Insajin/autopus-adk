package promptlayer

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildOMPContextPromotionStaticPolicyV3_BindsCanonicalReport(t *testing.T) {
	report := validOMPContextPromotionReportV1()
	policy, body, err := BuildOMPContextPromotionStaticPolicyV3(
		report,
		"darwin-arm64",
		OMPContextPromotionKeyID2026Q3K3,
		"release-lineage-2026-q3-k1",
		"v1",
		5096,
	)
	if err != nil {
		t.Fatalf("build static policy: %v", err)
	}
	if policy.SchemaVersion != OMPContextPromotionRuntimeSchemaV3 ||
		policy.ProducerRepository != report.Producer.Repository ||
		policy.ProducerWorkflowRef != report.Producer.WorkflowRef ||
		policy.CandidateRepository != report.Candidate.Repository ||
		policy.SourceCommit != report.Candidate.Revision ||
		policy.SourceTree != report.Candidate.TreeSHA ||
		policy.AutoVersion != report.Runtime.AutoVersion ||
		policy.OMPExecutableSHA256 != report.Runtime.OMPExecutableSHA256 ||
		policy.ProviderAuthorityDigest != OMPContextPromotionProviderAuthorityDigestV1(report) ||
		policy.CohortManifestDigest != report.CohortManifestDigest ||
		policy.OrderSeed != report.OrderSeed ||
		policy.PromotionSigningKeyID != OMPContextPromotionKeyID2026Q3K3 ||
		policy.MinimumRollbackFloor != 5096 {
		t.Fatalf("static policy does not bind canonical report: %#v", policy)
	}
	var decoded OMPContextPromotionStaticPolicyV3
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode static policy: %v", err)
	}
	if !reflect.DeepEqual(decoded, policy) {
		t.Fatalf("encoded static policy = %#v, want %#v", decoded, policy)
	}
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil ||
		fields["promotion_signing_key_id"] != OMPContextPromotionKeyID2026Q3K3 ||
		fields["provider_authority_digest"] != OMPContextPromotionProviderAuthorityDigestV1(report) {
		t.Fatalf("canonical static policy omitted active authority: %s", body)
	}
}

func TestMarshalOMPContextPromotionStaticPolicyV3_RejectsMissingProviderAuthority(t *testing.T) {
	policy, _, err := BuildOMPContextPromotionStaticPolicyV3(
		validOMPContextPromotionReportV1(), "darwin-arm64", OMPContextPromotionKeyID2026Q3K3,
		"release-lineage-2026-q3-k1", "v1", 5096,
	)
	if err != nil {
		t.Fatal(err)
	}
	policy.ProviderAuthorityDigest = ""
	if _, err := MarshalOMPContextPromotionStaticPolicyV3(policy); err == nil {
		t.Fatal("static policy without provider authority was serialized")
	}
}

func TestBuildOMPContextPromotionStaticPolicyV3_RejectsNonCanonicalOrUnsafeInput(t *testing.T) {
	report := validOMPContextPromotionReportV1()
	nonCanonical := report
	nonCanonical.OrderSeed = promotionSHA256([]byte("operator-selected-order"))
	if _, _, err := BuildOMPContextPromotionStaticPolicyV3(
		nonCanonical, "darwin-arm64", OMPContextPromotionKeyID2026Q3K3,
		"release-lineage-2026-q3-k1", "v1", 5096,
	); err == nil {
		t.Fatal("non-canonical report produced a static policy")
	}
	tests := []struct {
		name, target, promotionKeyID, lineageKeyID, handoff string
		floor                                               uint64
	}{
		{name: "target", promotionKeyID: OMPContextPromotionKeyID2026Q3K3, lineageKeyID: "release-lineage-2026-q3-k1", handoff: "v1", floor: 5096},
		{name: "empty promotion key", target: "darwin-arm64", lineageKeyID: "release-lineage-2026-q3-k1", handoff: "v1", floor: 5096},
		{name: "unknown promotion key", target: "darwin-arm64", promotionKeyID: "omp-context-promotion-unknown", lineageKeyID: "release-lineage-2026-q3-k1", handoff: "v1", floor: 5096},
		{name: "noncanonical promotion key", target: "darwin-arm64", promotionKeyID: " " + OMPContextPromotionKeyID2026Q3K3, lineageKeyID: "release-lineage-2026-q3-k1", handoff: "v1", floor: 5096},
		{name: "lineage key", target: "darwin-arm64", promotionKeyID: OMPContextPromotionKeyID2026Q3K3, handoff: "v1", floor: 5096},
		{name: "handoff", target: "darwin-arm64", promotionKeyID: OMPContextPromotionKeyID2026Q3K3, lineageKeyID: "release-lineage-2026-q3-k1", floor: 5096},
		{name: "floor", target: "darwin-arm64", promotionKeyID: OMPContextPromotionKeyID2026Q3K3, lineageKeyID: "release-lineage-2026-q3-k1", handoff: "v1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := BuildOMPContextPromotionStaticPolicyV3(
				report, test.target, test.promotionKeyID, test.lineageKeyID, test.handoff, test.floor,
			); err == nil {
				t.Fatal("unsafe release coordinate produced a static policy")
			}
		})
	}
}

func TestMarshalOMPContextPromotionStaticPolicyV3_AcceptsOnlyCommittedCanonicalSignerIDs(t *testing.T) {
	report := validOMPContextPromotionReportV1()
	for _, keyID := range []string{
		OMPContextPromotionKeyID2026Q3K1,
		OMPContextPromotionKeyID2026Q3K2,
		OMPContextPromotionKeyID2026Q3K3,
	} {
		if _, _, err := BuildOMPContextPromotionStaticPolicyV3(
			report, "darwin-arm64", keyID, "release-lineage-2026-q3-k1", "v1", 5096,
		); err != nil {
			t.Fatalf("committed signer %q rejected: %v", keyID, err)
		}
	}
}

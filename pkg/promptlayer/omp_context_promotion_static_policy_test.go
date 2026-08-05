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
		policy.CohortManifestDigest != report.CohortManifestDigest ||
		policy.OrderSeed != report.OrderSeed ||
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
}

func TestBuildOMPContextPromotionStaticPolicyV3_RejectsNonCanonicalOrUnsafeInput(t *testing.T) {
	report := validOMPContextPromotionReportV1()
	nonCanonical := report
	nonCanonical.OrderSeed = promotionSHA256([]byte("operator-selected-order"))
	if _, _, err := BuildOMPContextPromotionStaticPolicyV3(
		nonCanonical, "darwin-arm64", "release-lineage-2026-q3-k1", "v1", 5096,
	); err == nil {
		t.Fatal("non-canonical report produced a static policy")
	}
	for name, mutate := range map[string]func(*string, *string, *string, *uint64){
		"target":  func(target, _, _ *string, _ *uint64) { *target = "" },
		"key id":  func(_, keyID, _ *string, _ *uint64) { *keyID = "" },
		"handoff": func(_, _, handoff *string, _ *uint64) { *handoff = "" },
		"floor":   func(_, _, _ *string, floor *uint64) { *floor = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			target, keyID, handoff, floor := "darwin-arm64", "release-lineage-2026-q3-k1", "v1", uint64(5096)
			mutate(&target, &keyID, &handoff, &floor)
			if _, _, err := BuildOMPContextPromotionStaticPolicyV3(
				report, target, keyID, handoff, floor,
			); err == nil {
				t.Fatal("unsafe release coordinate produced a static policy")
			}
		})
	}
}

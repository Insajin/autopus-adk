package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func testStaticPolicyB64(t *testing.T) string {
	t.Helper()
	policy := promptlayer.OMPContextPromotionStaticPolicyV3{
		SchemaVersion:       promptlayer.OMPContextPromotionRuntimeSchemaV3,
		CandidateRepository: "Insajin/autopus-adk", SourceCommit: strings.Repeat("c", 40),
		SourceTree: strings.Repeat("d", 40), Target: "darwin-arm64",
		ReleaseLineageKeyID: "release-key", ReleaseLineageHandoff: "v1", MinimumRollbackFloor: 5093,
	}
	body, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(body)
}

func TestRun_RejectsUnpinnedEvidenceBeforeSignatureVerification(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	attestation := filepath.Join(dir, "attestation.json")
	if err := os.WriteFile(report, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attestation, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{
		"--mode", "historical", "--report", report, "--attestation", attestation,
		"--report-sha256", strings.Repeat("a", 64),
		"--attestation-sha256", strings.Repeat("b", 64),
		"--candidate-repository", "Insajin/autopus-adk",
		"--candidate-revision", strings.Repeat("c", 40),
		"--candidate-tree", strings.Repeat("d", 40),
		"--candidate-artifact-sha256", strings.Repeat("e", 64),
		"--static-policy-b64", testStaticPolicyB64(t),
	})
	if err == nil || !strings.Contains(err.Error(), "artifact digest differs") {
		t.Fatalf("unpinned evidence result = %v", err)
	}
}

func TestParseOptions_RejectsMalformedCoordinates(t *testing.T) {
	_, err := parseOptions([]string{
		"--mode", "active", "--report", "report", "--attestation", "attestation",
		"--report-sha256", strings.Repeat("a", 64),
		"--attestation-sha256", strings.Repeat("b", 64),
		"--candidate-repository", "Insajin/autopus-adk",
		"--candidate-revision", strings.Repeat("C", 40),
		"--candidate-tree", strings.Repeat("d", 40),
		"--candidate-artifact-sha256", strings.Repeat("e", 64),
		"--static-policy-b64", testStaticPolicyB64(t),
	})
	if err == nil || !strings.Contains(err.Error(), "candidate revision") {
		t.Fatalf("malformed coordinate result = %v", err)
	}
}

func TestDecodeStaticPolicy_RejectsUnknownAndNonCanonicalInput(t *testing.T) {
	valid := testStaticPolicyB64(t)
	policy, err := decodeStaticPolicy(valid)
	if err != nil || policy.Target != "darwin-arm64" {
		t.Fatalf("valid policy result = %#v, %v", policy, err)
	}
	body, err := base64.RawURLEncoding.DecodeString(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, encoded := range []string{
		base64.RawURLEncoding.EncodeToString(append([]byte(" "), body...)),
		base64.RawURLEncoding.EncodeToString(append(body[:len(body)-1], []byte(`,"unknown":true}`)...)),
		valid + "=",
	} {
		if _, err := decodeStaticPolicy(encoded); err == nil {
			t.Fatal("static policy wire drift was accepted")
		}
	}
}

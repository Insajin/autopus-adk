package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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
		ProducerRepository:  "Insajin/autopus-harness-bench",
		ProducerWorkflowRef: "refs/heads/main@0123456789abcdef0123456789abcdef01234567",
		CandidateRepository: "Insajin/autopus-adk", SourceCommit: strings.Repeat("c", 40),
		SourceTree: strings.Repeat("d", 40), Target: "darwin-arm64", AutoVersion: "v0.50.111",
		PolicyID: "omp-context-active-v1", PolicyDigest: promotionTestHash("policy"),
		OMPVersion: "omp/17.2.7", OMPExecutableSHA256: promotionTestHash("omp"),
		PipelineImplementationDigest: promotionTestHash("pipeline"),
		Provider:                     "openai", ProviderAuthorityDigest: promotionTestHash("provider-authority"),
		ModelScopeDigest:     promotionTestHash("models"),
		CohortManifestDigest: promotionTestHash("cohort"), OrderSeed: promotionTestHash("order"),
		OraclePolicyDigest:    promotionTestHash("oracle"),
		PromotionSigningKeyID: promptlayer.OMPContextPromotionKeyID2026Q3K3,
		ReleaseLineageKeyID:   "release-key", ReleaseLineageHandoff: "v1", MinimumRollbackFloor: 5093,
	}
	body, err := promptlayer.MarshalOMPContextPromotionStaticPolicyV3(policy)
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

func TestPromotionVerifierWrappers_RejectMalformedTamperedAndStaleEvidence(t *testing.T) {
	fixture := newHistoricalVerifierFixture(t)
	reportBytes, expected := promotionVerifierInputs(t, fixture)

	for name, verifier := range map[string]promotionArtifactVerifier{
		"active":     verifyActivePromotionArtifact,
		"historical": verifyHistoricalPromotionArtifact,
	} {
		t.Run(name+" malformed attestation", func(t *testing.T) {
			if valid, err := verifier(reportBytes, []byte("{}"), expected); err == nil || valid {
				t.Fatalf("%s wrapper accepted malformed attestation: valid=%v error=%v", name, valid, err)
			}
		})
		t.Run(name+" intrinsically stale attestation", func(t *testing.T) {
			keyID := promptlayer.OMPContextPromotionKeyID2026Q3K1
			verifierExpected := expected
			if name == "active" {
				keyID = promptlayer.OMPContextPromotionKeyID2026Q3K3
				verifierExpected.SigningKeyID = keyID
			}
			valid, err := verifier(reportBytes, stalePromotionAttestation(t, reportBytes, keyID), verifierExpected)
			if valid || !errors.Is(err, promptlayer.ErrOMPContextPromotionStale) {
				t.Fatalf("%s wrapper stale result: valid=%v error=%v", name, valid, err)
			}
		})
	}

	attestation := stalePromotionAttestation(t, reportBytes, promptlayer.OMPContextPromotionKeyID2026Q3K1)
	tamperedReport := append(append([]byte(nil), reportBytes...), ' ')
	if valid, err := verifyHistoricalPromotionArtifact(tamperedReport, attestation, expected); err == nil || valid {
		t.Fatalf("historical wrapper accepted modified report: valid=%v error=%v", valid, err)
	}
}

func TestRun_HistoricalAcceptsRawCandidateDigestAgainstCanonicalReport(t *testing.T) {
	fixture := newHistoricalVerifierFixture(t)
	policy, err := decodeStaticPolicy(fixture.staticPolicyB64)
	if err != nil {
		t.Fatal(err)
	}
	policy.PromotionSigningKeyID = promptlayer.OMPContextPromotionKeyID2026Q3K2
	policyBytes, err := promptlayer.MarshalOMPContextPromotionStaticPolicyV3(policy)
	if err != nil {
		t.Fatal(err)
	}
	fixture.staticPolicyB64 = base64.RawURLEncoding.EncodeToString(policyBytes)
	canonicalDigest := "sha256:" + strings.Repeat("e", 64)
	historicalVerifier := func(
		reportBytes, _ []byte,
		expected promptlayer.OMPContextPromotionExpectationV2,
	) (bool, error) {
		var report promptlayer.OMPContextPromotionReportV1
		if err := json.Unmarshal(reportBytes, &report); err != nil {
			t.Fatalf("decode report in verifier: %v", err)
		}
		if report.Candidate.ArtifactSHA256 != canonicalDigest ||
			expected.Candidate.ArtifactSHA256 != canonicalDigest ||
			expected.AutoBinarySHA256 != canonicalDigest ||
			expected.SigningKeyID != promptlayer.OMPContextPromotionKeyID2026Q3K2 {
			t.Fatalf("candidate digest boundary = report %q, candidate %q, runtime %q",
				report.Candidate.ArtifactSHA256, expected.Candidate.ArtifactSHA256,
				expected.AutoBinarySHA256)
		}
		return true, nil
	}
	if err := runWithVerifiers(fixture.arguments(strings.Repeat("e", 64)),
		verifyActivePromotionArtifact, historicalVerifier); err != nil {
		t.Fatalf("historical verification failed: %v", err)
	}
}

func TestRun_HistoricalRejectsMismatchedCandidateDigest(t *testing.T) {
	fixture := newHistoricalVerifierFixture(t)
	verifierCalled := false
	err := runWithVerifiers(
		fixture.arguments(strings.Repeat("f", 64)),
		verifyActivePromotionArtifact,
		func([]byte, []byte, promptlayer.OMPContextPromotionExpectationV2) (bool, error) {
			verifierCalled = true
			return true, nil
		},
	)
	if verifierCalled {
		t.Fatal("historical verifier called for mismatched candidate digest")
	}
	if err == nil || err.Error() != "candidate coordinates differ from exact release source" {
		t.Fatalf("mismatched candidate digest result = %v", err)
	}
}

func TestParseOptions_RejectsNonCanonicalCandidateArtifactDigest(t *testing.T) {
	for _, test := range []struct {
		name, value string
	}{
		{"prefixed", "sha256:" + strings.Repeat("e", 64)},
		{"uppercase", strings.Repeat("E", 64)},
		{"wrong length", strings.Repeat("e", 63)},
		{"non-hex", strings.Repeat("e", 63) + "g"},
		{"whitespace", " " + strings.Repeat("e", 63)},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseOptions([]string{
				"--mode", "historical", "--report", "report", "--attestation", "attestation",
				"--report-sha256", strings.Repeat("a", 64),
				"--attestation-sha256", strings.Repeat("b", 64),
				"--candidate-repository", "Insajin/autopus-adk",
				"--candidate-revision", strings.Repeat("c", 40),
				"--candidate-tree", strings.Repeat("d", 40),
				"--candidate-artifact-sha256", test.value,
				"--static-policy-b64", testStaticPolicyB64(t),
			})
			if err == nil || err.Error() != "candidate artifact digest is malformed" {
				t.Fatalf("non-canonical candidate digest result = %v", err)
			}
		})
	}
}

func TestParseOptions_RejectsExternalExpectedSigningKeyAuthority(t *testing.T) {
	arguments := []string{
		"--mode", "active", "--report", "report", "--attestation", "attestation",
		"--report-sha256", strings.Repeat("a", 64),
		"--attestation-sha256", strings.Repeat("b", 64),
		"--candidate-repository", "Insajin/autopus-adk",
		"--candidate-revision", strings.Repeat("c", 40),
		"--candidate-tree", strings.Repeat("d", 40),
		"--candidate-artifact-sha256", strings.Repeat("e", 64),
		"--static-policy-b64", testStaticPolicyB64(t),
		"--expected-signing-key-id", promptlayer.OMPContextPromotionKeyID2026Q3K3,
	}
	if _, err := parseOptions(arguments); err == nil || err.Error() != "invalid arguments" {
		t.Fatalf("external signing key authority result = %v", err)
	}
}

func TestRun_ActiveConsumesPolicyOwnedK3SigningKey(t *testing.T) {
	fixture := newHistoricalVerifierFixture(t)
	policy, err := decodeStaticPolicy(fixture.staticPolicyB64)
	if err != nil {
		t.Fatal(err)
	}
	policy.PromotionSigningKeyID = promptlayer.OMPContextPromotionKeyID2026Q3K3
	body, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	fixture.staticPolicyB64 = base64.RawURLEncoding.EncodeToString(body)
	arguments := fixture.arguments(strings.Repeat("e", 64))
	arguments[1] = "active"
	called := false
	err = runWithVerifiers(arguments, func(
		_ []byte, _ []byte, expected promptlayer.OMPContextPromotionExpectationV2,
	) (bool, error) {
		called = true
		if expected.SigningKeyID != promptlayer.OMPContextPromotionKeyID2026Q3K3 {
			t.Fatalf("active expectation signing key = %q", expected.SigningKeyID)
		}
		return true, nil
	}, verifyHistoricalPromotionArtifact)
	if err != nil || !called {
		t.Fatalf("policy-owned K3 result: called=%v error=%v", called, err)
	}
}

func TestRun_ActiveRejectsHistoricalPolicyKeysBeforeVerification(t *testing.T) {
	for _, keyID := range []string{
		promptlayer.OMPContextPromotionKeyID2026Q3K1,
		promptlayer.OMPContextPromotionKeyID2026Q3K2,
	} {
		t.Run(keyID, func(t *testing.T) {
			fixture := newHistoricalVerifierFixture(t)
			policy, err := decodeStaticPolicy(fixture.staticPolicyB64)
			if err != nil {
				t.Fatal(err)
			}
			policy.PromotionSigningKeyID = keyID
			body, err := json.Marshal(policy)
			if err != nil {
				t.Fatal(err)
			}
			fixture.staticPolicyB64 = base64.RawURLEncoding.EncodeToString(body)
			arguments := fixture.arguments(strings.Repeat("e", 64))
			arguments[1] = "active"
			called := false
			err = runWithVerifiers(arguments, func(
				[]byte, []byte, promptlayer.OMPContextPromotionExpectationV2,
			) (bool, error) {
				called = true
				return true, nil
			}, verifyHistoricalPromotionArtifact)
			if called || err == nil ||
				!strings.Contains(err.Error(), "active static policy is invalid") {
				t.Fatalf("historical key active result: called=%v error=%v", called, err)
			}
		})
	}
}

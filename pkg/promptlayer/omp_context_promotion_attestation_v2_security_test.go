package promptlayer

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestVerifyOMPContextPromotionArtifactV2_RejectsStrictJSONViolations(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	unknownReport := append(append([]byte(nil), fixture.reportBytes[:len(fixture.reportBytes)-1]...), []byte(`,"raw_body":"secret"}`)...)
	credentialReport := append(append([]byte(nil), fixture.reportBytes[:len(fixture.reportBytes)-1]...), []byte(`,"credential":"sk-test-SECRET"}`)...)
	duplicateReport := []byte(strings.Replace(string(fixture.reportBytes), `{"schema_version":`, `{"schema_version":"`+OMPContextPromotionReportSchemaV1+`","schema_version":`, 1))
	tests := []struct {
		name        string
		report      []byte
		attestation []byte
	}{
		{name: "malformed report", report: []byte(`{`), attestation: fixture.signRawReport(t, []byte(`{`))},
		{name: "duplicate report key", report: duplicateReport, attestation: fixture.signRawReport(t, duplicateReport)},
		{name: "trailing report", report: append(append([]byte(nil), fixture.reportBytes...), []byte(`{}`)...), attestation: fixture.signRawReport(t, append(append([]byte(nil), fixture.reportBytes...), []byte(`{}`)...))},
		{name: "unknown raw body", report: unknownReport, attestation: fixture.signRawReport(t, unknownReport)},
		{name: "credential field", report: credentialReport, attestation: fixture.signRawReport(t, credentialReport)},
		{name: "duplicate attestation key", report: fixture.reportBytes, attestation: []byte(strings.Replace(string(fixture.attestationBytes), `{"schema_version":`, `{"schema_version":"`+OMPContextPromotionAttestationSchemaV2+`","schema_version":`, 1))},
		{name: "trailing attestation", report: fixture.reportBytes, attestation: append(append([]byte(nil), fixture.attestationBytes...), []byte(`{}`)...)},
		{name: "unknown attestation field", report: fixture.reportBytes, attestation: addUnknownJSONField(t, fixture.attestationBytes, "private_key", "forbidden")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := verifyOMPContextPromotionArtifactV2WithTrust(test.report, test.attestation, fixture.now, fixture.expectation,
				map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil); err == nil {
				t.Fatal("strict JSON violation was accepted")
			}
		})
	}
}

func TestVerifyOMPContextPromotionArtifactV2_RejectsFreshnessAndRevocationViolations(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ompContextPromotionV2Fixture)
		revoked map[string]bool
	}{
		{name: "expired", mutate: func(f *ompContextPromotionV2Fixture) { f.attestation.ExpiresAt = f.now.Format(time.RFC3339Nano) }},
		{name: "future", mutate: func(f *ompContextPromotionV2Fixture) {
			f.attestation.IssuedAt = f.now.Add(6 * time.Minute).Format(time.RFC3339Nano)
			f.attestation.NotBefore = f.attestation.IssuedAt
			f.attestation.ExpiresAt = f.now.Add(time.Hour).Format(time.RFC3339Nano)
		}},
		{name: "before not-before", mutate: func(f *ompContextPromotionV2Fixture) {
			f.attestation.IssuedAt = f.now.Add(time.Minute).Format(time.RFC3339Nano)
			f.attestation.NotBefore = f.attestation.IssuedAt
			f.attestation.ExpiresAt = f.now.Add(time.Hour).Format(time.RFC3339Nano)
		}},
		{name: "excessive not-before backdate", mutate: func(f *ompContextPromotionV2Fixture) {
			f.attestation.NotBefore = f.now.Add(-7 * time.Minute).Format(time.RFC3339Nano)
		}},
		{name: "overlong ttl", mutate: func(f *ompContextPromotionV2Fixture) {
			f.attestation.ExpiresAt = f.now.Add(25 * time.Hour).Format(time.RFC3339Nano)
		}},
		{name: "revoked", mutate: func(*ompContextPromotionV2Fixture) {}, revoked: map[string]bool{OMPContextPromotionKeyID2026Q3K1: true}},
		{name: "bad signature", mutate: func(f *ompContextPromotionV2Fixture) {
			f.attestation.SignatureBase64 = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newOMPContextPromotionV2Fixture(t)
			test.mutate(fixture)
			fixture.signAttestation(t)
			if test.name == "bad signature" {
				fixture.attestation.SignatureBase64 = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
				fixture.attestationBytes, _ = json.Marshal(fixture.attestation)
			}
			if _, err := verifyOMPContextPromotionArtifactV2WithTrust(fixture.reportBytes, fixture.attestationBytes, fixture.now,
				fixture.expectation, map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, test.revoked); err == nil {
				t.Fatal("freshness or revocation violation was accepted")
			}
		})
	}
}

func TestVerifyOMPContextPromotionArtifactV2_AcceptsProducerMaximumValidityWindow(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	fixture.attestation.IssuedAt = fixture.now.Format(time.RFC3339Nano)
	fixture.attestation.NotBefore = fixture.now.Add(-5 * time.Minute).Format(time.RFC3339Nano)
	fixture.attestation.ExpiresAt = fixture.now.Add(24 * time.Hour).Format(time.RFC3339Nano)
	fixture.signAttestation(t)

	if _, err := verifyOMPContextPromotionArtifactV2WithTrust(
		fixture.reportBytes,
		fixture.attestationBytes,
		fixture.now,
		fixture.expectation,
		map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey},
		nil,
	); err != nil {
		t.Fatalf("producer maximum validity window rejected: %v", err)
	}
}

func TestVerifyOMPContextPromotionArtifactV2_RejectsFutureAndStaleCohorts(t *testing.T) {
	tests := []struct {
		name  string
		shift time.Duration
	}{
		{name: "future observations", shift: 3 * time.Hour},
		{name: "stale observations", shift: -time.Hour},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newOMPContextPromotionV2Fixture(t)
			for index := range fixture.report.Observations {
				observation := &fixture.report.Observations[index]
				started, err := time.Parse(time.RFC3339Nano, observation.StartedAt)
				if err != nil {
					t.Fatal(err)
				}
				completed, err := time.Parse(time.RFC3339Nano, observation.CompletedAt)
				if err != nil {
					t.Fatal(err)
				}
				observation.StartedAt = started.Add(test.shift).Format(time.RFC3339Nano)
				observation.CompletedAt = completed.Add(test.shift).Format(time.RFC3339Nano)
			}
			fixture.report.EvidenceID, _ = computeOMPContextPromotionEvidenceIDV1(fixture.report)
			fixture.resign(t)

			_, err := verifyOMPContextPromotionArtifactV2WithTrust(
				fixture.reportBytes,
				fixture.attestationBytes,
				fixture.now,
				fixture.expectation,
				map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey},
				nil,
			)
			if !errors.Is(err, ErrOMPContextPromotionStale) {
				t.Fatalf("cohort time violation returned %v, want stale", err)
			}
		})
	}
}

func TestVerifyOMPContextPromotionArtifactV2_RejectsReplayCoordinateMismatch(t *testing.T) {
	tests := map[string]func(*OMPContextPromotionExpectationV2){
		"producer repository": func(e *OMPContextPromotionExpectationV2) { e.ProducerRepository = "Insajin/other" },
		"producer workflow": func(e *OMPContextPromotionExpectationV2) {
			e.ProducerWorkflowRef = "refs/heads/main@1123456789abcdef0123456789abcdef01234567"
		},
		"signing key": func(e *OMPContextPromotionExpectationV2) {
			e.SigningKeyID = OMPContextPromotionKeyID2026Q3K2
		},
		"candidate": func(e *OMPContextPromotionExpectationV2) {
			e.Candidate.Revision = "2123456789abcdef0123456789abcdef01234567"
		},
		"tree": func(e *OMPContextPromotionExpectationV2) {
			e.Candidate.TreeSHA = "3123456789abcdef0123456789abcdef01234567"
		},
		"artifact": func(e *OMPContextPromotionExpectationV2) {
			e.Candidate.ArtifactSHA256 = promotionSHA256([]byte("other"))
		},
		"policy":  func(e *OMPContextPromotionExpectationV2) { e.PolicyDigest = promotionSHA256([]byte("other")) },
		"runtime": func(e *OMPContextPromotionExpectationV2) { e.OMPVersion = "omp/17.2.8" },
		"pipeline": func(e *OMPContextPromotionExpectationV2) {
			e.PipelineImplementationDigest = promotionSHA256([]byte("other"))
		},
		"provider": func(e *OMPContextPromotionExpectationV2) { e.Provider = "anthropic" },
		"cohort":   func(e *OMPContextPromotionExpectationV2) { e.CohortManifestDigest = promotionSHA256([]byte("other")) },
		"oracle":   func(e *OMPContextPromotionExpectationV2) { e.OraclePolicyDigest = promotionSHA256([]byte("other")) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newOMPContextPromotionV2Fixture(t)
			expected := fixture.expectation
			mutate(&expected)
			if _, err := verifyOMPContextPromotionArtifactV2WithTrust(fixture.reportBytes, fixture.attestationBytes, fixture.now,
				expected, map[string]ed25519.PublicKey{OMPContextPromotionKeyID2026Q3K1: fixture.publicKey}, nil); err == nil {
				t.Fatal("replayed coordinate was accepted")
			}
		})
	}
}

func (f *ompContextPromotionV2Fixture) signRawReport(t *testing.T, report []byte) []byte {
	t.Helper()
	attestation := f.attestation
	attestation.ReportSHA256 = promotionSHA256(report)
	message, err := ompContextPromotionAttestationMessageV2(attestation)
	if err != nil {
		t.Fatal(err)
	}
	attestation.SignatureBase64 = base64.StdEncoding.EncodeToString(ed25519.Sign(f.privateKey, message))
	body, err := json.Marshal(attestation)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func (f *ompContextPromotionV2Fixture) signAttestation(t *testing.T) {
	t.Helper()
	message, err := ompContextPromotionAttestationMessageV2(f.attestation)
	if err != nil {
		t.Fatal(err)
	}
	f.attestation.SignatureBase64 = base64.StdEncoding.EncodeToString(ed25519.Sign(f.privateKey, message))
	f.attestationBytes, err = json.Marshal(f.attestation)
	if err != nil {
		t.Fatal(err)
	}
}

func addUnknownJSONField(t *testing.T, body []byte, key string, value any) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	document[key] = value
	updated, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

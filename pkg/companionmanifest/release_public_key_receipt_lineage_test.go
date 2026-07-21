package companionmanifest

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA0Repository = "Insajin/autopus-adk"
	publicKeyReceiptA0Tag        = "v0.50.69"
	publicKeyReceiptA0Version    = "0.50.69"
	publicKeyReceiptA1Tag        = "v0.50.70"
	publicKeyReceiptA1Version    = "0.50.70"
	publicKeyReceiptA2Tag        = "v0.50.71"
	publicKeyReceiptA2Version    = "0.50.71"
	publicKeyReceiptA3Tag        = "v0.50.72"
	publicKeyReceiptA3Version    = "0.50.72"
	publicKeyReceiptA4Tag        = "v0.50.73"
	publicKeyReceiptA4Version    = "0.50.73"
	minimumLongLivedReceiptSecs  = "31536000"
)

func TestReleasePublicKeyReceipt_LineageClaims_UseConjunctiveLongLivedWindow(t *testing.T) {
	api := productionPublicKeyReceiptAPI(t)
	producer := normalizedReleaseText(releaseSourceFile(t, "scripts/companion-release/produce.sh"))
	if !strings.Contains(producer, "companion-manifest public-key-receipt") {
		t.Fatal("missing production contract: release producer does not issue a public-key receipt")
	}
	for _, shared := range []string{
		`--key-id "$COMPANION_KEY_ID"`,
		`--handoff "$COMPANION_HANDOFF"`,
		`--minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR"`,
		`--key-file "$COMPANION_SIGNING_KEY_FILE"`,
	} {
		if !strings.Contains(producer, shared) {
			t.Fatalf("public receipt does not share manifest key/handoff/floor claim %q", shared)
		}
	}
	for _, receiptWindow := range []string{
		`--issued-at "$COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT"`,
		`--expires-at "$COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT"`,
	} {
		if !strings.Contains(producer, receiptWindow) {
			t.Fatalf("public receipt lacks independent long-lived window argument %q", receiptWindow)
		}
	}
	if !strings.Contains(producer, api.flag) {
		t.Fatalf("receipt lineage is not emitted through secure CLI flag %s", api.flag)
	}
	releaseSources := normalizedReleaseText(releaseScriptsText(t))
	for _, oracle := range []string{
		"COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS",
		minimumLongLivedReceiptSecs,
		"receipt_window_not_long_lived",
		"manifest_window_outside_receipt",
		"manifest_public_key_digest_mismatch",
	} {
		if !strings.Contains(releaseSources, oracle) {
			t.Fatalf("missing conjunctive receipt/manifest release oracle %q", oracle)
		}
	}
}

func TestReleasePublicKeyReceipt_A0Policy_IsOneExactAuditableBootstrap(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	for _, exact := range []string{
		publicKeyReceiptA0Repository,
		publicKeyReceiptA0Tag,
		publicKeyReceiptA0Version,
		"GITHUB_REF_NAME",
		"COMPANION_VERSION",
		"release_phase='A0'",
		"release_phase='A1'",
		"release_phase='A2'",
		"release_phase='A3'",
		"release_phase='A4'", "release_phase='A5'", "release_phase='A6'", "release_phase='A7'", "release_phase='A8'", "release_phase='A9'", "release_phase='A10'", "release_phase='A11'",
	} {
		if !strings.Contains(scripts, exact) {
			t.Fatalf("missing exact A0 bootstrap release policy %q", exact)
		}
	}
	if !exactA0TagVersionGuard(scripts) {
		t.Fatal("A0 bootstrap is not conjunctively restricted to tag v0.50.69 and version 0.50.69")
	}
	if !exactA2TagVersionGuard(scripts) {
		t.Fatal("A2 release is not conjunctively restricted to tag v0.50.71 and version 0.50.71")
	}
	if !exactA3TagVersionGuard(scripts) {
		t.Fatal("A3 release is not conjunctively restricted to tag v0.50.72 and version 0.50.72")
	}
	if !exactA4TagVersionGuard(scripts) {
		t.Fatal("A4 release is not conjunctively restricted to tag v0.50.73 and version 0.50.73")
	}
	if !exactA5TagVersionGuard(scripts) {
		t.Fatal("A5 release is not conjunctively restricted to tag v0.50.74 and version 0.50.74")
	}
	if !exactA6TagVersionGuard(scripts) {
		t.Fatal("A6 release is not conjunctively restricted to tag v0.50.77 and version 0.50.77")
	}
	workflow := releaseWorkflowContract(t)
	for _, job := range workflow.Jobs {
		for _, step := range job.Steps {
			for name := range step.Env {
				upper := strings.ToUpper(name)
				if strings.Contains(upper, "BOOTSTRAP") || strings.Contains(upper, "IS_A0") || strings.Contains(upper, "ALLOW_A0") {
					t.Fatalf("caller-controlled %s can silently generalize A0 bootstrap", name)
				}
			}
		}
	}
}

func TestReleasePublicKeyReceipt_ExactA0Phase_AllowsUnprovisionedPinsWithCurrentContract(t *testing.T) {
	workflow := releaseWorkflowContract(t)
	validationIndex, validationStep := releaseWorkflowStepContaining(t, workflow, "validate-environment.sh")
	lineageIndex, lineageStep := publicKeyReceiptLineageStep(t, workflow)
	if validationIndex >= lineageIndex {
		t.Fatalf("current receipt contract validation step %d must precede A0 lineage step %d", validationIndex, lineageIndex)
	}
	requireCurrentReceiptContract(t, validationStep)
	requireCurrentReceiptContract(t, lineageStep)

	output, err := runPublicKeyReceiptLineagePhase(t, publicKeyReceiptA0Tag)
	if err != nil {
		t.Fatalf("exact A0 bootstrap rejected intentionally unprovisioned immutable pins: %v\n%s", err, output)
	}
	if !strings.Contains(output, "A0 bootstrap accepted") {
		t.Fatalf("exact A0 bootstrap output does not identify the audited phase: %q", output)
	}
}

func TestReleasePublicKeyReceipt_NonBootstrapPriorEvidence_VerifiesDirectPredecessorAndCurrentEquality(t *testing.T) {
	api := productionPublicKeyReceiptAPI(t)
	workflow := releaseWorkflowContract(t)
	lineageIndex, lineageStep := publicKeyReceiptLineageStep(t, workflow)
	releaseIndex, _ := releaseWorkflowStepContaining(t, workflow, "goreleaser release --clean")
	if lineageIndex >= releaseIndex {
		t.Fatalf("A1 immutable A0 evidence gate runs at step %d, not before GoReleaser step %d", lineageIndex, releaseIndex)
	}
	if !strings.Contains(lineageStep.Run, "scripts/companion-release/") {
		t.Fatalf("A1 lineage step does not execute a reviewed production verifier: %q", lineageStep.Run)
	}
	scripts := normalizedReleaseText(releaseScriptsText(t))
	for _, required := range []string{
		"gh api", "gh release download", "releases/tags/",
		publicKeyReceiptA0Repository, publicKeyReceiptA0Tag, publicKeyReceiptA1Tag, publicKeyReceiptA2Tag, publicKeyReceiptA3Tag, publicKeyReceiptA4Tag, publicKeyReceiptA5Tag,
		publicKeyReceiptA6Tag, publicKeyReceiptA7Tag, publicKeyReceiptA8Tag, publicKeyReceiptA9Tag, publicKeyReceiptA10Tag, publicKeyReceiptA11Tag,
		"tag_name", "target_commitish", "cmp --",
		"prior_receipt", "current_receipt", "record_sha256", "public_key_sha256",
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A1 immutable prior-release verification missing %q", required)
		}
	}
	wantComparisons := 2
	if !api.bundle {
		wantComparisons = 1
	}
	if got := strings.Count(scripts, "cmp --"); got < wantComparisons {
		t.Fatalf("A1 exact receipt/signature-or-envelope byte comparisons = %d, want at least %d", got, wantComparisons)
	}
	requirePublicKeyReceiptLineagePhaseFailure(t, "v0.50.75", "prior_release_identity_mismatch")
	requirePublicKeyReceiptLineagePhaseFailure(t, "v0.50.76", "prior_release_identity_mismatch")
	requirePublicKeyReceiptLineagePhaseFailure(t, "v0.50.83", "prior_release_identity_mismatch")
	for _, tag := range []string{
		publicKeyReceiptA11Tag, publicKeyReceiptA10Tag, publicKeyReceiptA9Tag, publicKeyReceiptA8Tag, publicKeyReceiptA7Tag,
		publicKeyReceiptA6Tag, publicKeyReceiptA5Tag, publicKeyReceiptA4Tag, publicKeyReceiptA3Tag, publicKeyReceiptA2Tag,
	} {
		requirePublicKeyReceiptLineagePhaseFailure(t, tag, "prior_evidence_unverifiable")
	}
	if !a0LineagePinsProvisioned(t, scripts, api) {
		requirePublicKeyReceiptLineagePhaseFailure(t, "v0.50.70", "prior_evidence_unverifiable")
	}
}

func TestReleasePublicKeyReceipt_NonBootstrapPriorEvidenceFailures_BlockBeforeGoReleaser(t *testing.T) {
	workflow := releaseWorkflowContract(t)
	lineageIndex, _, lineageFound := findPublicKeyReceiptLineageStep(workflow)
	releaseIndex, _ := releaseWorkflowStepContaining(t, workflow, "goreleaser release --clean")
	if !lineageFound {
		t.Error("missing production contract: non-bootstrap release has no prior-evidence preflight before GoReleaser")
	} else if lineageIndex >= releaseIndex {
		t.Fatalf("non-bootstrap evidence gate index %d must precede publish index %d", lineageIndex, releaseIndex)
	}
	scripts := normalizedReleaseText(releaseScriptsText(t))
	api := productionPublicKeyReceiptAPI(t)
	signedBytesFailure := "prior_signature_bytes_mismatch"
	if !api.bundle {
		signedBytesFailure = "prior_envelope_bytes_mismatch"
	}
	cases := []struct {
		name string
		code string
	}{
		{name: "absent", code: "prior_evidence_absent"},
		{name: "malformed", code: "prior_evidence_malformed"},
		{name: "release_identity_mismatch", code: "prior_release_identity_mismatch"},
		{name: "receipt_bytes_mismatch", code: "prior_receipt_bytes_mismatch"},
		{name: "signed_bytes_mismatch", code: signedBytesFailure},
		{name: "record_digest_mismatch", code: "prior_record_digest_mismatch"},
		{name: "public_key_digest_mismatch", code: "prior_public_key_digest_mismatch"},
		{name: "unverifiable", code: "prior_evidence_unverifiable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(scripts, tc.code) {
				t.Fatalf("missing fail-closed production error %q before GoReleaser", tc.code)
			}
		})
	}
}

func releaseScriptsText(t *testing.T) []byte {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "scripts", "companion-release", "*.sh"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("find companion release scripts: %v", err)
	}
	sort.Strings(paths)
	var combined strings.Builder
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read release script %s: %v", path, readErr)
		}
		combined.WriteString("\n# source: " + filepath.Base(path) + "\n")
		combined.Write(data)
	}
	return []byte(combined.String())
}

func exactA0TagVersionGuard(source string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`GITHUB_REF_NAME.{0,240}v0\.50\.69.{0,400}COMPANION_VERSION.{0,240}0\.50\.69`),
		regexp.MustCompile(`COMPANION_VERSION.{0,240}0\.50\.69.{0,400}GITHUB_REF_NAME.{0,240}v0\.50\.69`),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(source) {
			return true
		}
	}
	return false
}

func exactA2TagVersionGuard(source string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`GITHUB_REF_NAME.{0,240}v0\.50\.71.{0,400}COMPANION_VERSION.{0,240}0\.50\.71`),
		regexp.MustCompile(`COMPANION_VERSION.{0,240}0\.50\.71.{0,400}GITHUB_REF_NAME.{0,240}v0\.50\.71`),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(source) {
			return true
		}
	}
	return false
}

func exactA3TagVersionGuard(source string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`GITHUB_REF_NAME.{0,240}v0\.50\.72.{0,400}COMPANION_VERSION.{0,240}0\.50\.72`),
		regexp.MustCompile(`COMPANION_VERSION.{0,240}0\.50\.72.{0,400}GITHUB_REF_NAME.{0,240}v0\.50\.72`),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(source) {
			return true
		}
	}
	return false
}

func publicKeyReceiptLineageStep(t *testing.T, workflow publicKeyReceiptWorkflow) (int, publicKeyReceiptWorkflowStep) {
	t.Helper()
	index, step, ok := findPublicKeyReceiptLineageStep(workflow)
	if !ok {
		t.Fatal("missing production contract: release workflow has no prior-release receipt lineage preflight")
	}
	return index, step
}

func findPublicKeyReceiptLineageStep(workflow publicKeyReceiptWorkflow) (int, publicKeyReceiptWorkflowStep, bool) {
	job, ok := workflow.Jobs["release"]
	if !ok {
		return -1, publicKeyReceiptWorkflowStep{}, false
	}
	for index, step := range job.Steps {
		surface := strings.ToLower(step.Name + " " + step.Run)
		if strings.Contains(surface, "public-key") &&
			(strings.Contains(surface, "lineage") || strings.Contains(surface, "prior")) {
			return index, step, true
		}
	}
	return -1, publicKeyReceiptWorkflowStep{}, false
}

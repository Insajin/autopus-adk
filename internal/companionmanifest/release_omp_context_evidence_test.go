package companionmanifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflow_A22GatesOnPinnedOMPProductionEvidence(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml") + "\n" +
		readReleaseFile(t, "scripts/companion-release/verify-omp-context-evidence-tag.sh")
	for _, required := range []string{
		"omp-production-evidence:",
		"permissions:\n      contents: read",
		"needs: [ci, security, omp-production-evidence]",
		"omp-context-evidence-v0.50.100",
		"OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA",
		"OMP_CONTEXT_EVIDENCE_COMMIT_SHA",
		"OMP_CONTEXT_EVIDENCE_TREE_SHA",
		"OMP_CONTEXT_EVIDENCE_REPORT_SHA256",
		"OMP_CONTEXT_EVIDENCE_ATTESTATION_SHA256",
		"scripts/companion-release/verify-omp-context-evidence-tag.sh",
		"./scripts/companion-release/ompcontextverify",
		"--mode active",
		"--mode historical",
		"--expected-signing-key-id 'omp-context-promotion-2026-q3-k2'",
		"omp-context-promotion-report.v1.json",
		"omp-context-promotion-attestation.v2.json",
	} {
		if !strings.Contains(release, required) {
			t.Fatalf("A22 production evidence workflow missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"environment:\n      name: omp-context-promotion",
		"OMP_CONTEXT_PROMOTION_ED25519_PRIVATE_KEY_PEM",
		"id-token: write\n    runs-on: ubuntu",
	} {
		if strings.Contains(release, forbidden) {
			t.Fatalf("no-secret evidence gate contains forbidden authority %q", forbidden)
		}
	}
}

func TestReleaseWorkflow_A22CarriesSameRunEvidenceIntoExactFifteenAssets(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	config := readReleaseFile(t, ".goreleaser.yaml")
	for _, required := range []string{
		"actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02",
		"actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093",
		"retention-days: 1",
		"OMP_CONTEXT_PROMOTION_REPORT_PATH",
		"OMP_CONTEXT_PROMOTION_ATTESTATION_PATH",
		"OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256",
		"OMP_CONTEXT_STATIC_POLICY_B64",
		"static-policy-sha256",
		"OMP_CONTEXT_RELEASE_LINEAGE_PATH",
		"OMP_CONTEXT_RELEASE_LINEAGE_SIGNATURE_PATH",
		"verify-omp-context-release-binary.sh",
	} {
		if !strings.Contains(release+"\n"+config, required) {
			t.Fatalf("A22 same-run evidence contract missing %q", required)
		}
	}
	for _, required := range []string{
		`glob: "{{ .Env.OMP_CONTEXT_PROMOTION_REPORT_PATH }}"`,
		`glob: "{{ .Env.OMP_CONTEXT_PROMOTION_ATTESTATION_PATH }}"`,
		`glob: "{{ .Env.OMP_CONTEXT_RELEASE_LINEAGE_PATH }}"`,
		`glob: "{{ .Env.OMP_CONTEXT_RELEASE_LINEAGE_SIGNATURE_PATH }}"`,
		`eq .Version "0.50.100"`,
		"dst: release-lineage-v1.json",
		"dst: release-lineage-v1.sig",
	} {
		if !strings.Contains(config, required) {
			t.Fatalf("GoReleaser extra evidence asset missing %q", required)
		}
	}
	current := readReleaseFile(t, "scripts/companion-release/verify-current-release.sh")
	for _, required := range []string{
		"exactly fifteen A22 release assets",
		"OMP_CONTEXT_EVIDENCE_VERIFIER",
		"OMP_CONTEXT_LINEAGE_VERIFIER",
		"COMPANION_MANIFEST_VERIFIER",
		"OMP_CONTEXT_STATIC_POLICY_B64",
		"release-lineage-v1.json",
		"release-lineage-v1.sig",
		"standalone and archived OMP context lineage bytes differ",
		"--static-policy-b64",
		"--expected-signing-key-id",
		"omp-context-promotion-2026-q3-k2",
	} {
		if !strings.Contains(current, required) {
			t.Fatalf("current release verifier missing V3/fifteen-asset contract %q", required)
		}
	}
}

func TestOMPContextEvidenceVerifier_UsesDistinctPublicTrustAPIs(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/ompcontextverify/main.go")
	for _, required := range []string{
		"promptlayer.VerifyOMPContextPromotionArtifactV2",
		"promptlayer.VerifyOMPContextPromotionHistoricalArtifactV2",
		"candidate artifact digest differs from runtime binary digest",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("OMP evidence verifier missing %q", required)
		}
	}
}

func TestOMPContextLineageCleanup_RemovesOnlyOwnedEnabledOutputs(t *testing.T) {
	root := repositoryRoot(t)
	lineage := filepath.Join(t.TempDir(), "release-lineage-v1.json")
	signature := filepath.Join(filepath.Dir(lineage), "release-lineage-v1.sig")
	for _, path := range []string{lineage, signature} {
		if err := os.WriteFile(path, []byte("preexisting"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	helper := filepath.Join(root, "scripts", "companion-release", "produce-omp-context-lineage.sh")
	script := `source "$1"
omp_context_lineage_enabled=0 lineage_path=$2 lineage_signature_path=$3
cleanup_omp_context_release_lineage
test -f "$2" && test -f "$3"
omp_context_lineage_enabled=1
cleanup_omp_context_release_lineage
test ! -e "$2" && test ! -e "$3"`
	command := exec.Command("/bin/bash", "-c", script, "bash", helper, lineage, signature)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("lineage cleanup ownership: %v\n%s", err, output)
	}
}

func TestOMPContextEvidenceTagVerifier_NegativeMatrix(t *testing.T) {
	root := repositoryRoot(t)
	contract := filepath.Join(root, "scripts", "companion-release", "tests",
		"release-omp-context-evidence-hardening-test.sh")
	command := exec.Command("/bin/bash", contract)
	command.Dir = root
	command.Env = os.Environ()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("OMP context evidence hardening: %v\n%s", err, output)
	}
}

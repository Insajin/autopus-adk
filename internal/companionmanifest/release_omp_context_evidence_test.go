package companionmanifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflow_A22UsesIndependentlyAuthorizedCanonicalBridge(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	sidecar := readReleaseFile(t, "scripts/companion-release/verify-key-rotation-sidecar.sh")
	for _, required := range []string{
		"omp-canonical-bridge-candidate:",
		"needs: [ci, security, omp-canonical-bridge-candidate]",
		"verify-key-rotation-sidecar.sh",
		"materialize-key-rotation-authority.sh",
		"key-rotation-authority/verify-rotation.sh",
		"refs/heads/release-key-rotation-v0.50.109",
		"adk-key-rotation-v1.json",
		"adk-key-rotation-v1.sig",
		"verify-rotation",
		"release-tag-signing-2026-q3-r2.pub",
		"release-tag-signing-2026-q3-r2.fingerprint",
		"ADK_KEY_ROTATION_VERIFIED=1",
		"canonical-full-bridge",
	} {
		if !strings.Contains(release+"\n"+sidecar, required) {
			t.Fatalf("A22 canonical bridge authority missing %q", required)
		}
	}
	if strings.Contains(release, "./internal/adkchannel/cmd") {
		t.Fatal("release workflow rebuilds candidate-controlled rotation verifier")
	}
	for _, forbidden := range []string{
		"omp-context-evidence-v0.50.109",
		"OMP_CONTEXT_EVIDENCE_",
		"OMP_CONTEXT_STATIC_POLICY_B64",
		"--mode active",
		"--mode historical",
		"omp-context-promotion-2026-q3-k2",
	} {
		if strings.Contains(release, forbidden) {
			t.Fatalf("A22 bridge workflow contains forbidden promotion authority %q", forbidden)
		}
	}
}

func TestReleaseWorkflow_A22PublishesBridgeAndRotationAssets(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	config := readReleaseFile(t, ".goreleaser.yaml")
	current := readReleaseFile(t, "scripts/companion-release/verify-current-release.sh")
	for _, required := range []string{
		"OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256",
		"OMP_CONTEXT_BRIDGE_MANIFEST_PATH",
		"OMP_CONTEXT_RELEASE_LINEAGE_PATH",
		"ADK_KEY_ROTATION_DOCUMENT_PATH",
		"ADK_KEY_ROTATION_SIGNATURE_PATH",
		"ADK_KEY_ROTATION_VERIFIER",
		"OMP_CONTEXT_RELEASE_LINEAGE_SIGNATURE_PATH",
		"verify-omp-context-release-binary.sh",
		"omp-context-bridge-release.v1.json",
		"exactly sixteen A22 canonical-full bridge assets",
		"adk-key-rotation-v1.json",
		"adk-key-rotation-v1.sig",
		"verify-rotation-historical",
		"OMP_CONTEXT_LINEAGE_VERIFIER",
		"COMPANION_MANIFEST_VERIFIER",
		"standalone and archived lineage bytes differ",
		"canonical-full-bridge",
	} {
		if !strings.Contains(release+"\n"+config+"\n"+current, required) {
			t.Fatalf("A22 bridge release evidence contract missing %q", required)
		}
	}
	for _, required := range []string{
		`glob: "{{ .Env.OMP_CONTEXT_BRIDGE_MANIFEST_PATH }}"`,
		`glob: "{{ .Env.ADK_KEY_ROTATION_DOCUMENT_PATH }}"`,
		`glob: "{{ .Env.ADK_KEY_ROTATION_SIGNATURE_PATH }}"`,
		`glob: "{{ .Env.OMP_CONTEXT_RELEASE_LINEAGE_PATH }}"`,
		`glob: "{{ .Env.OMP_CONTEXT_RELEASE_LINEAGE_SIGNATURE_PATH }}"`,
		`{{ if and (eq .Os "darwin") (eq .Arch "arm64") }}dist/auto_{{ .Target }}/release-lineage-v1.json`,
		`{{ if and (eq .Os "darwin") (eq .Arch "arm64") }}dist/auto_{{ .Target }}/release-lineage-v1.sig`,
		"dst: release-lineage-v1.json",
		"dst: release-lineage-v1.sig",
	} {
		if !strings.Contains(config, required) {
			t.Fatalf("GoReleaser bridge evidence asset missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"pipelineOMPActiveStaticPolicyB64",
		"OMP_CONTEXT_STATIC_POLICY_B64",
		"OMP_CONTEXT_PROMOTION_REPORT_PATH",
		"OMP_CONTEXT_PROMOTION_ATTESTATION_PATH",
		"omp-context-promotion-report.v1.json",
		"omp-context-promotion-attestation.v2.json",
	} {
		if strings.Contains(release+"\n"+config, forbidden) {
			t.Fatalf("bridge release contains forbidden active-promotion wiring %q", forbidden)
		}
	}
	if strings.Contains(config, "eq .Version") {
		t.Fatal("GoReleaser gates a release asset on a per-release version coordinate")
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

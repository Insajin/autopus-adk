package companionmanifest

import (
	"strings"
	"testing"
)

const a22A21AncestorSHA = "b86fab067599f457261287552c5a9dd86460d7f4"

func TestReleaseSourceValidator_A22AcceptsAnnotatedA21DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A22 release candidate", "v0.50.93")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.93", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A22 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A22") ||
		!strings.Contains(output, "source-tree="+tree) {
		t.Fatalf("validated A22 output = %q", output)
	}
}

func TestReleaseSourceValidator_A22PinsDirectA21Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A22_A21_ANCESTOR_SHA='" + a22A21AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A22 immutable A21 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`v0.50.93) release_phase='A22'`,
		`git merge-base --is-ancestor "$A22_A21_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`fail 'A22 source does not contain the immutable A21 release'`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A22 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A22PinsExactImmutableA21Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A21_RELEASE_ID='363679709'",
		"A21_COMMIT_SHA='b86fab067599f457261287552c5a9dd86460d7f4'",
		"A21_TREE_SHA='64d32460859b325e55dcf9959a8478567c8849a7'",
		"A21_TAG_OBJECT_SHA='fdc0e94fbe2325ed47cf7a3fd1792028c1f3bb75'",
		"A21_CHECKSUMS_SHA256='ae2933e24be05002d02497cc56fa4bb48c3b366941e8888b4874ed3445c9f892'",
		"A21_AMD64_ARCHIVE_SHA256='027587eb7e7a73a33541ac4e9aaed1d36b446298ecf48ca78292ee87dc589f58'",
		"A21_ARM64_ARCHIVE_SHA256='4364bb3e3db09df8ba63927c8f8f64978fd63d880831dfc154f4ac9a92484492'",
		"A21_LINUX_AMD64_ARCHIVE_SHA256='3058d0eec967a341c5eb9a176315f5093ba8a3763db6bb88d78df69ad2e4c961'",
		"A21_LINUX_ARM64_ARCHIVE_SHA256='a259b171cb5da9ad7ff6bae2d23f7e222297ff443456530928741bae118dbdc7'",
		"A21_AMD64_MANIFEST_SHA256='4ceb4c9fd59e53295a7846c5d018cb2b65798d6aba6a91c9b3b5373c1342dde9'",
		"A21_ARM64_MANIFEST_SHA256='5ab616a51d76c2606347caa0aa334467c21508e3f917307e264def17e58c5503'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A22 prior evidence pin drifted: %s", declaration)
		}
	}
}

func TestLineageVerifier_A22UsesExactA21ReleaseCoordinate(t *testing.T) {
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	for _, required := range []string{
		"A21_REPOSITORY='Insajin/autopus-adk' A22_TAG='v0.50.93' A22_VERSION='0.50.93'",
		"release_phase='A22' prior_phase='A21'",
		`prior_release_id="$A21_RELEASE_ID"`,
		`prior_tree="$A21_TREE_SHA"`,
		`prior_linux_amd64_archive="$A21_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A21_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A22 lineage coordinate contract missing %q", required)
		}
	}
}

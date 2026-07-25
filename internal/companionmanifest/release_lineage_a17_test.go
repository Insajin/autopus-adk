package companionmanifest

import (
	"fmt"
	"strings"
	"testing"
)

const a17A16AncestorSHA = "3e02c622af97f74873325ec65940c580e23c580a"

func TestReleaseSourceValidator_A17AcceptsAnnotatedA16DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A17 release candidate", "v0.50.88")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.88", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A17 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A17") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A17 output = %q", output)
	}
}

func TestReleaseSourceValidator_A17RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.88")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.88", sha)
		if err == nil || !strings.Contains(output, "A17 release tag must be annotated") {
			t.Fatalf("lightweight A17 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A16", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A17", "v0.50.88")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.88", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A16 release") {
			t.Fatalf("A16-free A17 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A17 release candidate", "v0.50.88")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.88", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A17 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A17PinsDirectA16Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A17_A16_ANCESTOR_SHA='" + a17A16AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A17 immutable A16 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A17_A16_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A17 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A17PinsExactImmutableA16Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A16_COMMIT_SHA='3e02c622af97f74873325ec65940c580e23c580a'",
		"A16_TREE_SHA='f041a63af99e2ba82bd6fb573272489fbd101e86'",
		"A16_TAG_OBJECT_SHA='9fad744950e268fa500812cad0194fa5da47e369'",
		"A16_CHECKSUMS_SHA256='7c19613a7f441264f90fb6b0c6e975c78cae3db250f2ed279fa44418a022dc7f'",
		"A16_AMD64_ARCHIVE_SHA256='cc411eb9cc04476d280d272f0e52b7c1a40fad923439180b526a46c38be1b63e'",
		"A16_ARM64_ARCHIVE_SHA256='b0880ef40f3089168be234a540cfaf1795e0e54168a4ead3f92275a13eb63012'",
		"A16_LINUX_AMD64_ARCHIVE_SHA256='8da3ec03967fa1b5911708716239bcaa9d0843069e65836f280f986b4cdd1aaa'",
		"A16_LINUX_ARM64_ARCHIVE_SHA256='9aeca632be6de54d3540e03ad99ba5dc520f3665f7f592bd12b689b844ea8bf3'",
		"A16_AMD64_MANIFEST_SHA256='93943fafac83eb3090f10f8c48489a96b80d003c5c9f00dfaa2cd59eabf7be42'",
		"A16_ARM64_MANIFEST_SHA256='4cc59ac1a3194df80a050e4159a6919a01e2bcc69efc575907aedf16aacc4057'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A17 prior evidence pin drifted: %s", declaration)
		}
	}
	for phase := 0; phase <= 13; phase++ {
		if strings.Contains(pins, fmt.Sprintf("A%d_LINUX_", phase)) {
			t.Fatalf("historical A%d Linux pin must remain absent", phase)
		}
	}
}

func TestLineageVerifier_A17UsesFourPinnedArchivesAndTwoDarwinManifests(t *testing.T) {
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	assets := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-assets.sh")
	for _, required := range []string{
		"A16_REPOSITORY='Insajin/autopus-adk' A17_TAG='v0.50.88' A17_VERSION='0.50.88'",
		"release_phase='A17' prior_phase='A16'",
		`prior_evidence_source='immutable A16 GitHub release'`,
		`prior_tree="$A16_TREE_SHA"`, `prior_linux_amd64_archive="$A16_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A16_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A17 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
		`source "$assets_helper"`, "verify_public_key_lineage_assets",
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A17 lineage caller contract missing %q", required)
		}
	}
	for _, required := range []string{
		"_darwin_amd64.tar.gz", "_darwin_arm64.tar.gz",
		"_linux_amd64.tar.gz", "_linux_arm64.tar.gz",
		`actual_asset_digest" == "$asset_digest`,
		`actual_asset_digest" == "sha256:$archive_pin`,
		`sha256_file "$download_dir/$asset`,
		`extract_bundle "$download_dir/$darwin_amd64_asset`,
		`extract_bundle "$download_dir/$darwin_arm64_asset`,
	} {
		if !strings.Contains(assets, required) {
			t.Fatalf("A17 lineage asset contract missing %q", required)
		}
	}
	if strings.Count(assets, "extract_bundle ") != 2 {
		t.Fatal("only the two Darwin archives may be extracted as manifest bundles")
	}
}

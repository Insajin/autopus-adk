package companionmanifest

import (
	"fmt"
	"strings"
	"testing"
)

const a19A18AncestorSHA = "76f35d990e76511d169e239547d33bfedcea7948"

func TestReleaseSourceValidator_A19AcceptsAnnotatedA18DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A19 release candidate", "v0.50.90")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.90", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A19 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A19") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A19 output = %q", output)
	}
}

func TestReleaseSourceValidator_A19RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.90")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.90", sha)
		if err == nil || !strings.Contains(output, "A19 release tag must be annotated") {
			t.Fatalf("lightweight A19 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A18", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A19", "v0.50.90")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.90", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A18 release") {
			t.Fatalf("A18-free A19 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A19 release candidate", "v0.50.90")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.90", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A19 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A19PinsDirectA18Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A19_A18_ANCESTOR_SHA='" + a19A18AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A19 immutable A18 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A19_A18_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A19 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A19PinsExactImmutableA18Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A18_RELEASE_ID='359747783'",
		"A18_COMMIT_SHA='76f35d990e76511d169e239547d33bfedcea7948'",
		"A18_TREE_SHA='5e90442a2b6dec38ec0072117d24b1d5ecb1f1e1'",
		"A18_TAG_OBJECT_SHA='ba18d42068693111ed78b4bc1ccc4891d0663334'",
		"A18_CHECKSUMS_SHA256='027f4b6a1b7412f19aafa386c18b9b551096e99a3dfdd44e2ad7b550952eab30'",
		"A18_AMD64_ARCHIVE_SHA256='4101bc35739f6dd428bdafe9e450624430690687dc0635e4e7dc97b62dd344a6'",
		"A18_ARM64_ARCHIVE_SHA256='d89d77a8618136643f26c3920f0c5d8aa4f87d5c48f2e5d555f3caf072fc375c'",
		"A18_LINUX_AMD64_ARCHIVE_SHA256='79f54ed935a204c6f5a89a93ba8945e8ffe15557080a1dc25eac437a16398782'",
		"A18_LINUX_ARM64_ARCHIVE_SHA256='aa4834873453266c853213d940cde4b672f6c00de9672718fcb8004582076b5f'",
		"A18_AMD64_MANIFEST_SHA256='71ef53e32e6e9fbfd1c03dd130412742f854bebb7958978ca8e2345e918b0a1e'",
		"A18_ARM64_MANIFEST_SHA256='a554b5a75cfd5c5f04ed9c06a6a2e1ba49220c2025e0149b030ed89a68757c56'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A19 prior evidence pin drifted: %s", declaration)
		}
	}
	for phase := 0; phase <= 13; phase++ {
		if strings.Contains(pins, fmt.Sprintf("A%d_LINUX_", phase)) {
			t.Fatalf("historical A%d Linux pin must remain absent", phase)
		}
	}
}

func TestLineageVerifier_A19UsesExactReleaseIDAndFourPinnedArchives(t *testing.T) {
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	assets := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-assets.sh")
	for _, required := range []string{
		"A18_REPOSITORY='Insajin/autopus-adk' A19_TAG='v0.50.90' A19_VERSION='0.50.90'",
		"release_phase='A19' prior_phase='A18'",
		`prior_evidence_source='immutable A18 GitHub release'`,
		`prior_release_id="$A18_RELEASE_ID"`, `prior_tree="$A18_TREE_SHA"`,
		`prior_linux_amd64_archive="$A18_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A18_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A19 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.id' "$release_json")" == "$prior_release_id" ]]`,
		`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
		`source "$assets_helper"`, "verify_public_key_lineage_assets",
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A19 lineage caller contract missing %q", required)
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
			t.Fatalf("A19 lineage asset contract missing %q", required)
		}
	}
	if strings.Count(assets, "extract_bundle ") != 2 {
		t.Fatal("only the two Darwin archives may be extracted as manifest bundles")
	}
}

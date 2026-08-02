package companionmanifest

import (
	"fmt"
	"strings"
	"testing"
)

const a21A20AncestorSHA = "7f44e4f143b2348c02553bab2209088c966f81ae"

func TestReleaseSourceValidator_A21AcceptsAnnotatedA20DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A21 release candidate", "v0.50.92")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.92", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A21 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A21") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A21 output = %q", output)
	}
}

func TestReleaseSourceValidator_A21RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.92")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.92", sha)
		if err == nil || !strings.Contains(output, "A21 release tag must be annotated") {
			t.Fatalf("lightweight A21 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A20", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A21", "v0.50.92")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.92", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A20 release") {
			t.Fatalf("A20-free A21 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A21 release candidate", "v0.50.92")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.92", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A21 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A21PinsDirectA20Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A21_A20_ANCESTOR_SHA='" + a21A20AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A21 immutable A20 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A21_A20_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A21 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A21PinsExactImmutableA20Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A20_RELEASE_ID='360327495'",
		"A20_COMMIT_SHA='7f44e4f143b2348c02553bab2209088c966f81ae'",
		"A20_TREE_SHA='8fdf3615e40f5e81512517619d4225ee067f8c23'",
		"A20_TAG_OBJECT_SHA='60158446cbf0599317b2850609d5e0d957a965db'",
		"A20_CHECKSUMS_SHA256='ff546eed918bc35dc420451341da563f2d6487c3a6565f9058dbc99362943f5a'",
		"A20_AMD64_ARCHIVE_SHA256='97a9fc7c6da9348709e3dbc48cb8587551e9980d61a33566103c844abdd3f05c'",
		"A20_ARM64_ARCHIVE_SHA256='baff6e099c8debc97bdf61bf0ea98813e2bcdef58f5171cad67edb84bd64b548'",
		"A20_LINUX_AMD64_ARCHIVE_SHA256='1011d88f66658f2b9ebe2caefb536038266005f229f846de2f1e597b17e25ea6'",
		"A20_LINUX_ARM64_ARCHIVE_SHA256='2905bc851e4637c60fd198f42331a7a6bf13180ee917ca67d9f20affce436c5d'",
		"A20_AMD64_MANIFEST_SHA256='d34a841d2dbca009e7ae7af2780ed8792e2501b04f30b0711d197093ea61a24c'",
		"A20_ARM64_MANIFEST_SHA256='2c08f77641786a8afe9e94de8734cb7a318868d3189e336ef01b704cca0e40d0'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A21 prior evidence pin drifted: %s", declaration)
		}
	}
	for phase := 0; phase <= 13; phase++ {
		if strings.Contains(pins, fmt.Sprintf("A%d_LINUX_", phase)) {
			t.Fatalf("historical A%d Linux pin must remain absent", phase)
		}
	}
}

func TestLineageVerifier_A21UsesExactReleaseIDAndFourPinnedArchives(t *testing.T) {
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	assets := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-assets.sh")
	for _, required := range []string{
		"A20_REPOSITORY='Insajin/autopus-adk' A21_TAG='v0.50.92' A21_VERSION='0.50.92'",
		"release_phase='A21' prior_phase='A20'",
		`prior_evidence_source='immutable A20 GitHub release'`,
		`prior_release_id="$A20_RELEASE_ID"`, `prior_tree="$A20_TREE_SHA"`,
		`prior_linux_amd64_archive="$A20_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A20_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A21 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.id' "$release_json")" == "$prior_release_id" ]]`,
		`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
		`source "$assets_helper"`, "verify_public_key_lineage_assets",
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A21 lineage caller contract missing %q", required)
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
			t.Fatalf("A21 lineage asset contract missing %q", required)
		}
	}
	if strings.Count(assets, "extract_bundle ") != 2 {
		t.Fatal("only the two Darwin archives may be extracted as manifest bundles")
	}
}

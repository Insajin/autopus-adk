package companionmanifest

import (
	"fmt"
	"strings"
	"testing"
)

const a15A14AncestorSHA = "4b8eb62200d253b46e022670c482e2f716a992a3"

func TestReleaseSourceValidator_A15AcceptsAnnotatedA14DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A15 release candidate", "v0.50.86")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.86", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A15 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A15") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A15 output = %q", output)
	}
}

func TestReleaseSourceValidator_A15RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.86")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.86", sha)
		if err == nil || !strings.Contains(output, "A15 release tag must be annotated") {
			t.Fatalf("lightweight A15 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A14", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A15", "v0.50.86")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.86", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A14 release") {
			t.Fatalf("A14-free A15 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A15 release candidate", "v0.50.86")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.86", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A15 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A15PinsDirectA14Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A15_A14_ANCESTOR_SHA='" + a15A14AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A15 immutable A14 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A15_A14_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A15 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A15PinsExactImmutableA14Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A14_COMMIT_SHA='4b8eb62200d253b46e022670c482e2f716a992a3'",
		"A14_TREE_SHA='fbdc83287982899c3d6bfe5fdf7b88494e76bcb0'",
		"A14_TAG_OBJECT_SHA='f005dd935dbbcec8c60052adcfda6632fe8831e1'",
		"A14_CHECKSUMS_SHA256='5bd11e327eab31c555f89298761e2d27bca2fadebfc3b7961cafb6a140539236'",
		"A14_AMD64_ARCHIVE_SHA256='66834d509309cb09b84f78bb81a97e68a8d03434c9a37f239a2ae04677dbc10b'",
		"A14_ARM64_ARCHIVE_SHA256='7fe10bc7b03b3df44f803622e3830e5e91f3ea12b47b706cf14f716b076b012e'",
		"A14_LINUX_AMD64_ARCHIVE_SHA256='187620011ce035f6bdb09f3f6d5b005f878463c3ba0fd805142cbd3e4f587698'",
		"A14_LINUX_ARM64_ARCHIVE_SHA256='654e42612a3f1ee670157cd461b3dff1270f2102b085984951975c0284356172'",
		"A14_AMD64_MANIFEST_SHA256='4265d3f18c7aaab779a720216c2f1dfc9a486c01be898290d4f56be31102008e'",
		"A14_ARM64_MANIFEST_SHA256='918c91d4bdee0c58e74e0068314d35463e094fef214986a550579bca08b2ef38'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A15 prior evidence pin drifted: %s", declaration)
		}
	}
	for phase := 0; phase <= 13; phase++ {
		if strings.Contains(pins, fmt.Sprintf("A%d_LINUX_", phase)) {
			t.Fatalf("historical A%d Linux pin must remain absent", phase)
		}
	}
}

func TestLineageVerifier_A15UsesFourPinnedArchivesAndTwoDarwinManifests(t *testing.T) {
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	assets := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-assets.sh")
	for _, required := range []string{
		"A14_REPOSITORY='Insajin/autopus-adk' A15_TAG='v0.50.86' A15_VERSION='0.50.86'",
		"release_phase='A15' prior_phase='A14'",
		`prior_evidence_source='immutable A14 GitHub release'`,
		`prior_tree="$A14_TREE_SHA"`, `prior_linux_amd64_archive="$A14_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A14_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A15 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
		`source "$assets_helper"`, "verify_public_key_lineage_assets",
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A15 lineage caller contract missing %q", required)
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
			t.Fatalf("A15 lineage asset contract missing %q", required)
		}
	}
	if strings.Count(assets, "extract_bundle ") != 2 {
		t.Fatal("only the two Darwin archives may be extracted as manifest bundles")
	}
}

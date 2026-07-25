package companionmanifest

import (
	"fmt"
	"strings"
	"testing"
)

const a18A17AncestorSHA = "2b062a5e348fbecc414abe9ba5c74c7dc79fe243"

func TestReleaseSourceValidator_A18AcceptsAnnotatedA17DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A18 release candidate", "v0.50.89")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.89", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A18 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A18") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A18 output = %q", output)
	}
}

func TestReleaseSourceValidator_A18RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.89")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.89", sha)
		if err == nil || !strings.Contains(output, "A18 release tag must be annotated") {
			t.Fatalf("lightweight A18 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A17", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A18", "v0.50.89")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.89", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A17 release") {
			t.Fatalf("A17-free A18 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A18 release candidate", "v0.50.89")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.89", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A18 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A18PinsDirectA17Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A18_A17_ANCESTOR_SHA='" + a18A17AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A18 immutable A17 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A18_A17_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A18 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A18PinsExactImmutableA17Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A17_RELEASE_ID='359675749'",
		"A17_COMMIT_SHA='2b062a5e348fbecc414abe9ba5c74c7dc79fe243'",
		"A17_TREE_SHA='1e376104c94748a546607abce6667813163d3d61'",
		"A17_TAG_OBJECT_SHA='8721b6be61a058aa5987727d3bae6d87c5fb38f4'",
		"A17_CHECKSUMS_SHA256='a4b437c11a5aaad986a4e0b93b0db0e19c3a49a1a801ba25e8b096c462bf3c47'",
		"A17_AMD64_ARCHIVE_SHA256='3bae06adc5c31281efd8d9fd016af0ab9158da54262e7f17fafd2ed9be6ff1b1'",
		"A17_ARM64_ARCHIVE_SHA256='bf7a96a4ce34b58a940ab9813c2b797ec8f4a489c794b54d6b54097c5ecc4cce'",
		"A17_LINUX_AMD64_ARCHIVE_SHA256='e09b66b7a1683ef8764e59ca620242ee7eebbee573c0fec5232f9adb649f6909'",
		"A17_LINUX_ARM64_ARCHIVE_SHA256='31cc659ae347346204db2dae13368e6277b1e21f7e3a7c16dab160dd2b98176d'",
		"A17_AMD64_MANIFEST_SHA256='56b4b53840ee7c859077245ff6ddc95ef0ea530f581b3831aa6ee8645b9a9749'",
		"A17_ARM64_MANIFEST_SHA256='457b6fef8ebcda7b3977a0786f2397a30591225ac5112858aab5ba920176df8c'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A18 prior evidence pin drifted: %s", declaration)
		}
	}
	for phase := 0; phase <= 13; phase++ {
		if strings.Contains(pins, fmt.Sprintf("A%d_LINUX_", phase)) {
			t.Fatalf("historical A%d Linux pin must remain absent", phase)
		}
	}
}

func TestLineageVerifier_A18UsesExactReleaseIDAndFourPinnedArchives(t *testing.T) {
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	assets := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-assets.sh")
	for _, required := range []string{
		"A17_REPOSITORY='Insajin/autopus-adk' A18_TAG='v0.50.89' A18_VERSION='0.50.89'",
		"release_phase='A18' prior_phase='A17'",
		`prior_evidence_source='immutable A17 GitHub release'`,
		`prior_release_id="$A17_RELEASE_ID"`, `prior_tree="$A17_TREE_SHA"`,
		`prior_linux_amd64_archive="$A17_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A17_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A18 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.id' "$release_json")" == "$prior_release_id" ]]`,
		`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
		`source "$assets_helper"`, "verify_public_key_lineage_assets",
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A18 lineage caller contract missing %q", required)
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
			t.Fatalf("A18 lineage asset contract missing %q", required)
		}
	}
	if strings.Count(assets, "extract_bundle ") != 2 {
		t.Fatal("only the two Darwin archives may be extracted as manifest bundles")
	}
}

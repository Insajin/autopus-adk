package companionmanifest

import (
	"fmt"
	"strings"
	"testing"
)

const a20A19AncestorSHA = "5bc41dccc72f8244943fd9e862cba07a36bf09d3"

func TestReleaseSourceValidator_A20AcceptsAnnotatedA19DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A20 release candidate", "v0.50.91")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.91", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A20 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A20") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A20 output = %q", output)
	}
}

func TestReleaseSourceValidator_A20RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.91")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.91", sha)
		if err == nil || !strings.Contains(output, "A20 release tag must be annotated") {
			t.Fatalf("lightweight A20 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A19", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A20", "v0.50.91")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.91", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A19 release") {
			t.Fatalf("A19-free A20 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A20 release candidate", "v0.50.91")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.91", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A20 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A20PinsDirectA19Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A20_A19_ANCESTOR_SHA='" + a20A19AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A20 immutable A19 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A20_A19_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A20 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A20PinsExactImmutableA19Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A19_RELEASE_ID='359989008'",
		"A19_COMMIT_SHA='5bc41dccc72f8244943fd9e862cba07a36bf09d3'",
		"A19_TREE_SHA='d7d26e5f66ac5d01556fb64eadd07407e00b5035'",
		"A19_TAG_OBJECT_SHA='d4e2a495d194da97594e3eb6504f9ad3533b0dd8'",
		"A19_CHECKSUMS_SHA256='3ac2b6563fd9800beed1cf682ef8d0af4a246041e31bed9eec4c62431ab269e4'",
		"A19_AMD64_ARCHIVE_SHA256='8fe1e6b04ee107ebb3af2de1e0b3a272f42ab352f15e62ef1a2ab9e0b067c630'",
		"A19_ARM64_ARCHIVE_SHA256='60c4fe79c83a4b58482808a6ea9937389b9fa5c27015521df76705db98fd42b0'",
		"A19_LINUX_AMD64_ARCHIVE_SHA256='34582ef39c7b54fb9327e295e6cbb50107e3689120cf74f75b1f21a01eecc1dc'",
		"A19_LINUX_ARM64_ARCHIVE_SHA256='11a00378365069b97804d6af40435645f26271550d1ab58e7776346ec7d4a879'",
		"A19_AMD64_MANIFEST_SHA256='c5383760f8d914870c9a5db4d00116beeda68f03be0a0967b35e2b6ec2823c55'",
		"A19_ARM64_MANIFEST_SHA256='41996a1a4b5c550cac02ac10f1e426a38f24555a0968fbc85807fad500f8dd9e'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A20 prior evidence pin drifted: %s", declaration)
		}
	}
	for phase := 0; phase <= 13; phase++ {
		if strings.Contains(pins, fmt.Sprintf("A%d_LINUX_", phase)) {
			t.Fatalf("historical A%d Linux pin must remain absent", phase)
		}
	}
}

func TestLineageVerifier_A20UsesExactReleaseIDAndFourPinnedArchives(t *testing.T) {
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	assets := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-assets.sh")
	for _, required := range []string{
		"A19_REPOSITORY='Insajin/autopus-adk' A20_TAG='v0.50.91' A20_VERSION='0.50.91'",
		"release_phase='A20' prior_phase='A19'",
		`prior_evidence_source='immutable A19 GitHub release'`,
		`prior_release_id="$A19_RELEASE_ID"`, `prior_tree="$A19_TREE_SHA"`,
		`prior_linux_amd64_archive="$A19_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A19_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A20 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.id' "$release_json")" == "$prior_release_id" ]]`,
		`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
		`source "$assets_helper"`, "verify_public_key_lineage_assets",
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A20 lineage caller contract missing %q", required)
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
			t.Fatalf("A20 lineage asset contract missing %q", required)
		}
	}
	if strings.Count(assets, "extract_bundle ") != 2 {
		t.Fatal("only the two Darwin archives may be extracted as manifest bundles")
	}
}

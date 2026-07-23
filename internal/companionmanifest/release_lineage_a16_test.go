package companionmanifest

import (
	"fmt"
	"strings"
	"testing"
)

const a16A15AncestorSHA = "0fc4f60dac8ff8afe69b680c8bf723bfbced4769"

func TestReleaseSourceValidator_A16AcceptsAnnotatedA15DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A16 release candidate", "v0.50.87")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.87", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A16 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A16") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A16 output = %q", output)
	}
}

func TestReleaseSourceValidator_A16RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.87")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.87", sha)
		if err == nil || !strings.Contains(output, "A16 release tag must be annotated") {
			t.Fatalf("lightweight A16 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A15", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A16", "v0.50.87")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.87", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A15 release") {
			t.Fatalf("A15-free A16 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A16 release candidate", "v0.50.87")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.87", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A16 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A16PinsDirectA15Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A16_A15_ANCESTOR_SHA='" + a16A15AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A16 immutable A15 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A16_A15_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A16 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A16PinsExactImmutableA15Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A15_COMMIT_SHA='0fc4f60dac8ff8afe69b680c8bf723bfbced4769'",
		"A15_TREE_SHA='3daa4aef3528338439acb34f50d3b4a19ababea5'",
		"A15_TAG_OBJECT_SHA='bb24ad6a554beee871063070b219b409245c0e93'",
		"A15_CHECKSUMS_SHA256='237f985675f866c234a41066735a2bff3ae0b554a2fe1b1b6b57aed125bac8f7'",
		"A15_AMD64_ARCHIVE_SHA256='41e2a371c89567ff862d5f47179c838cb3aefd83abeb0ff769e58b12579676e3'",
		"A15_ARM64_ARCHIVE_SHA256='84ea326a10c860af82663db1c87a8a15bdee492143d77a02ad86a0b3ba930f8f'",
		"A15_LINUX_AMD64_ARCHIVE_SHA256='cae69dd8828cb2c12ba0d312c3f4dbc034104c1b4b9cee6cddf18eebe6430cb6'",
		"A15_LINUX_ARM64_ARCHIVE_SHA256='9e943908dabf910e9f3072f838a99dec3c9d4952d9058bfbc1b71cd78e3f29eb'",
		"A15_AMD64_MANIFEST_SHA256='c2398cd51093cb19804ef2d07e1848cc77d16610a4669e78e0e1577a466df300'",
		"A15_ARM64_MANIFEST_SHA256='83da7620c878841c06980ad12315023dc2054b71f27cb7dfb53931a4224d0099'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A16 prior evidence pin drifted: %s", declaration)
		}
	}
	for phase := 0; phase <= 13; phase++ {
		if strings.Contains(pins, fmt.Sprintf("A%d_LINUX_", phase)) {
			t.Fatalf("historical A%d Linux pin must remain absent", phase)
		}
	}
}

func TestLineageVerifier_A16UsesFourPinnedArchivesAndTwoDarwinManifests(t *testing.T) {
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	assets := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-assets.sh")
	for _, required := range []string{
		"A15_REPOSITORY='Insajin/autopus-adk' A16_TAG='v0.50.87' A16_VERSION='0.50.87'",
		"release_phase='A16' prior_phase='A15'",
		`prior_evidence_source='immutable A15 GitHub release'`,
		`prior_tree="$A15_TREE_SHA"`, `prior_linux_amd64_archive="$A15_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A15_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A16 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
		`source "$assets_helper"`, "verify_public_key_lineage_assets",
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A16 lineage caller contract missing %q", required)
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
			t.Fatalf("A16 lineage asset contract missing %q", required)
		}
	}
	if strings.Count(assets, "extract_bundle ") != 2 {
		t.Fatal("only the two Darwin archives may be extracted as manifest bundles")
	}
}

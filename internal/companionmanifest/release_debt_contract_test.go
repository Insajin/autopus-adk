package companionmanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @AX:NOTE [AUTO]: The 250-line ceiling preserves A16 expansion headroom below the 300-line source limit.
const releaseDebtHeadroomLimit = 250

func TestReleaseDebtSaturatedScriptsHaveExpansionHeadroom(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"scripts/companion-release/verify-public-key-lineage.sh",
		"scripts/companion-release/verify-public-key-lineage-assets.sh",
		"scripts/companion-release/publish-homebrew-formula-bridge.sh",
		"scripts/companion-release/tests/release-hardening-test.sh",
		"scripts/companion-release/tests/release-lineage-pins-hardening-test.sh",
		"scripts/companion-release/produce.sh",
		"scripts/companion-release/produce-public-key-receipt.sh",
		"pkg/companionmanifest/release_public_key_receipt_lineage_test.go",
	} {
		data, err := os.ReadFile(filepath.Join(repositoryRoot(t), path))
		if err != nil {
			t.Fatal(err)
		}
		if lines := strings.Count(string(data), "\n") + 1; lines > releaseDebtHeadroomLimit {
			t.Errorf("%s has %d lines, want <= %d for A16 headroom",
				path, lines, releaseDebtHeadroomLimit)
		}
	}
}

func TestReleaseDebtProducerReceiptHasDedicatedHelper(t *testing.T) {
	t.Parallel()

	producer := readReleaseFile(t, "scripts/companion-release/produce.sh")
	helper := readReleaseFile(t,
		"scripts/companion-release/produce-public-key-receipt.sh")
	for _, required := range []string{
		"produce-public-key-receipt.sh",
		`[[ -f "$public_key_receipt_helper" && ! -L "$public_key_receipt_helper"`,
		`source "$public_key_receipt_helper"`,
		"resolve_public_key_receipt_release_phase produce_public_key_receipt_bundle",
	} {
		if !strings.Contains(producer, required) {
			t.Fatalf("producer receipt helper gate missing %q", required)
		}
	}
	for _, required := range []string{
		"release_phase='A0'", "release_phase='A16'",
		"companion-manifest public-key-receipt",
		"public key receipt independent verification failed",
		"manifest_public_key_digest_mismatch",
	} {
		if !strings.Contains(helper, required) {
			t.Fatalf("producer receipt helper missing %q", required)
		}
	}
	if strings.Contains(producer, "release_phase='A16'") ||
		strings.Contains(producer, "companion-manifest public-key-receipt") {
		t.Fatal("producer caller still owns receipt phase coordinates or publication")
	}
}

func TestReleaseDebtLineageCoordinatesHaveDedicatedHelper(t *testing.T) {
	t.Parallel()

	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	for _, required := range []string{
		"verify-public-key-lineage-coordinates.sh",
		`[[ -f "$coordinates_helper" && ! -L "$coordinates_helper" ]]`,
		`source "$coordinates_helper"`,
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("lineage helper gate missing %q", required)
		}
	}
	for _, required := range []string{
		"release_phase='A0'", "release_phase='A16' prior_phase='A15'",
		"prior_tree=", "prior_release_identity_mismatch",
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("lineage coordinate helper missing %q", required)
		}
	}
	if strings.Contains(lineage, "release_phase='A16' prior_phase='A15'") {
		t.Fatal("lineage caller still owns the A16 coordinate table")
	}
}

func TestReleaseDebtLineageAssetsHaveDedicatedHelper(t *testing.T) {
	t.Parallel()

	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	assets := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-assets.sh")
	for _, required := range []string{
		"verify-public-key-lineage-assets.sh",
		`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
		`source "$assets_helper"`, "verify_public_key_lineage_assets",
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("lineage asset helper gate missing %q", required)
		}
	}
	for _, required := range []string{
		"_darwin_amd64.tar.gz", "_darwin_arm64.tar.gz",
		"_linux_amd64.tar.gz", "_linux_arm64.tar.gz",
		`actual_asset_digest" == "$asset_digest`,
		`actual_asset_digest" == "sha256:$archive_pin`,
		"extract_bundle", "checksums.txt differs from its",
	} {
		if !strings.Contains(assets, required) {
			t.Fatalf("lineage asset helper missing %q", required)
		}
	}
	if strings.Contains(lineage, "gh release download") ||
		strings.Contains(lineage, "archive differs from checksums.txt") {
		t.Fatal("lineage caller still owns predecessor asset verification")
	}
}

func TestReleaseDebtHistoricalPinsHaveDedicatedHardeningScript(t *testing.T) {
	t.Parallel()

	aggregator := readReleaseFile(t,
		"scripts/companion-release/tests/release-hardening-test.sh")
	history := readReleaseFile(t,
		"scripts/companion-release/tests/release-lineage-pins-hardening-test.sh")
	if !strings.Contains(aggregator,
		`bash "$tests_dir/release-lineage-pins-hardening-test.sh"`) {
		t.Fatal("release hardening aggregator does not invoke lineage pin assertions")
	}
	for _, required := range []string{
		"A6_A5_ANCESTOR_SHA", "A16_A15_ANCESTOR_SHA",
		"A4_TAG_OBJECT_SHA", "A15_LINUX_ARM64_ARCHIVE_SHA256",
		"symlinked lineage asset helper passed",
	} {
		if !strings.Contains(history, required) {
			t.Fatalf("lineage pin hardening helper missing %q", required)
		}
	}
	if strings.Contains(aggregator, "A13_CHECKSUMS_SHA256") {
		t.Fatal("release hardening aggregator still owns historical pin assertions")
	}
}

func TestReleaseDebtHomebrewCASHasDedicatedHelper(t *testing.T) {
	t.Parallel()

	bridge := readReleaseFile(t,
		"scripts/companion-release/publish-homebrew-formula-bridge.sh")
	gitHelper := readReleaseFile(t,
		"scripts/companion-release/publish-homebrew-formula-bridge-git.sh")
	for _, required := range []string{
		"publish-homebrew-formula-bridge-git.sh",
		`[[ -f "$git_helper" && ! -L "$git_helper" ]]`,
		`source "$git_helper"`,
	} {
		if !strings.Contains(bridge, required) {
			t.Fatalf("Homebrew helper gate missing %q", required)
		}
	}
	for _, required := range []string{
		"verify_frozen_formula", "verify_prior_tap_head", "publish_cask",
		"api_json POST 'git/blobs'", "api_json POST 'git/trees'",
		"api_json POST 'git/commits'", `api_json PATCH "git/refs/heads/${TAP_BRANCH}"`,
	} {
		if !strings.Contains(gitHelper, required) {
			t.Fatalf("Homebrew CAS helper missing %q", required)
		}
	}
	if strings.Contains(bridge, "publish_cask()") ||
		strings.Contains(bridge, "verify_frozen_formula()") {
		t.Fatal("Homebrew caller still owns Git CAS or Formula freeze behavior")
	}
}

func TestReleaseDebtRuntimeAssertionsHaveDedicatedScript(t *testing.T) {
	t.Parallel()

	hardening := readReleaseFile(t,
		"scripts/companion-release/tests/release-hardening-test.sh")
	runtime := readReleaseFile(t,
		"scripts/companion-release/tests/release-runtime-hardening-test.sh")
	if !strings.Contains(hardening, `bash "$tests_dir/release-runtime-hardening-test.sh"`) {
		t.Fatal("release hardening aggregator does not invoke runtime assertions")
	}
	for _, required := range []string{
		"run_source_gate", "expired manifest window passed",
		"unanchored self-signed receipt passed", "tampered manifest signature passed",
	} {
		if !strings.Contains(runtime, required) {
			t.Fatalf("release runtime hardening helper missing %q", required)
		}
	}
	if strings.Contains(hardening, "go build -trimpath -o") {
		t.Fatal("release hardening aggregator still owns executable runtime assertions")
	}
}

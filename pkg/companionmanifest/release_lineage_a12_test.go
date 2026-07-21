package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA12Tag     = "v0.50.83"
	publicKeyReceiptA12Version = "0.50.83"
)

var immutableA11LineagePins = map[string]string{
	"A11_COMMIT_SHA":            "a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9",
	"A11_TREE_SHA":              "9545ed7437e6dfd7573952586a31964061e30e2d",
	"A11_TAG_OBJECT_SHA":        "c636f42a6e8dc65ef6500eb95dac4ef7d1faff9a",
	"A11_CHECKSUMS_SHA256":      "a7973f9fa27d1e0ca1d1943adcfe5be0fa6807ba0517ff9066b2659fa6f4f01c",
	"A11_AMD64_ARCHIVE_SHA256":  "f5825b4aff8ce84e6b18dfb0ae0249a432a1b247477c3a9e2cd14689a405d40d",
	"A11_ARM64_ARCHIVE_SHA256":  "c913c51b396e01034e889f43ef4da68fcae851e7f7cba7f2b8ac60a2c4e00c66",
	"A11_AMD64_MANIFEST_SHA256": "5a036574b0cfe8fa62dfe3dde3d65d248ed225aa883c898caced3d55906b47ba",
	"A11_ARM64_MANIFEST_SHA256": "990b9f1cfb0768db4bb23719006320d845b72322fa9fddc2317ab75381b734ee",
}

func TestReleasePublicKeyReceipt_A12PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA12TagVersionGuard(scripts) {
		t.Fatal("A12 release is not conjunctively restricted to tag v0.50.83 and version 0.50.83")
	}
	for _, required := range []string{
		"release_phase='A12'", "prior_phase='A11'", `prior_tree="$A11_TREE_SHA"`,
		`'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A12 exact A11 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA12FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA12Tag, publicKeyReceiptA12Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchiveFile(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A12 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.83"`)) {
			t.Fatalf("A12 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A12 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA11FixtureSealsA12DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA11Tag, publicKeyReceiptA11Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A11 to A12 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A12 exact A11 key record verified") {
			t.Fatalf("valid A12 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A11 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A11 annotated tag object differs from its pin", tamper: tamperA12TagObject},
		{name: "source_tree", code: "A11 source tree differs from its pin", tamper: tamperA12TreePin},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA12ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA12ManifestPin},
		{name: "receipt_continuity", code: "prior_receipt_bytes_mismatch", tamper: tamperLineageReceiptPin},
		{name: "signature_continuity", code: "prior_signature_bytes_mismatch", tamper: tamperLineageSignaturePin},
		{name: "record_continuity", code: "prior_record_digest_mismatch", tamper: tamperLineageRecordPin},
		{name: "commit_pin", code: "prior_release_identity_mismatch", tamper: tamperLineageCommitPin},
		{name: "tag_commit", code: "prior_release_identity_mismatch", tamper: tamperLineageTagCommit},
		{name: "target_commit", code: "prior_release_identity_mismatch", tamper: tamperLineageTargetCommit},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExecutableLineageFixture(t, tools, evidence)
			test.tamper(t, fixture)
			output, err := fixture.run(t)
			if err == nil {
				t.Fatalf("tampered A12 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A12 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A12 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA12TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA12TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA12ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA12ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA12TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "83")
}

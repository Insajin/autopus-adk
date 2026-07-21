package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA10Tag     = "v0.50.81"
	publicKeyReceiptA10Version = "0.50.81"
)

var immutableA9LineagePins = map[string]string{
	"A9_COMMIT_SHA":            "c9c4f49d48022eb0c8d72ee7b520136a4f21f176",
	"A9_TREE_SHA":              "3a71fa56bd917f447a6b1705772b6ab99bbcfbc8",
	"A9_TAG_OBJECT_SHA":        "b7d05fa76eed41b1dfb4eddbd9873525e0aac15f",
	"A9_CHECKSUMS_SHA256":      "9ed1f99d22a761abb7953c70aab3c7de5ab0b7ec3524cf3798fcd3815c53bde7",
	"A9_AMD64_ARCHIVE_SHA256":  "48f80577ff2ef40a843dab0a847895ca7b3877e7fb810a30d328cbe8a55fc51e",
	"A9_ARM64_ARCHIVE_SHA256":  "503c338e1ce122e209b9e74bc883492317144b319b0713943bc299e57447024d",
	"A9_AMD64_MANIFEST_SHA256": "589f02503aa02338ed14d67b1eb6b31e2b96a9e83b47c99e5cd5a31b75ede9b7",
	"A9_ARM64_MANIFEST_SHA256": "ffdd6ccbecff2b8ea38bc5c5f65ff7f078b229bd4658f90d08bb5e801c184a7f",
}

func TestReleasePublicKeyReceipt_A10PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA10TagVersionGuard(scripts) {
		t.Fatal("A10 release is not conjunctively restricted to tag v0.50.81 and version 0.50.81")
	}
	for _, required := range []string{
		"release_phase='A10'", "prior_phase='A9'", `prior_tree="$A9_TREE_SHA"`,
		`'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A10 exact A9 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA10FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA10Tag, publicKeyReceiptA10Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchive(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A10 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.81"`)) {
			t.Fatalf("A10 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A10 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA9FixtureSealsA10DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA9Tag, publicKeyReceiptA9Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A9 to A10 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A10 exact A9 key record verified") {
			t.Fatalf("valid A10 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A9 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A9 annotated tag object differs from its pin", tamper: tamperA10TagObject},
		{name: "source_tree", code: "A9 source tree differs from its pin", tamper: tamperA10TreePin},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA10ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA10ManifestPin},
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
				t.Fatalf("tampered A10 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A10 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A10 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA10TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA10TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA10ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA10ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA10TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "81")
}

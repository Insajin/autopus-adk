package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA8Tag     = "v0.50.79"
	publicKeyReceiptA8Version = "0.50.79"
)

var immutableA7LineagePins = map[string]string{
	"A7_COMMIT_SHA":            "51de6030a69a8e36fcf7e5790ef157eff6fedf00",
	"A7_TREE_SHA":              "3cd00b17bd8bd6aa8def213de1c5765c3611765d",
	"A7_TAG_OBJECT_SHA":        "417a318fb6a11a720e2c4102e92e39ea9ed676e9",
	"A7_CHECKSUMS_SHA256":      "322d2ef21dff55f02ca36944aba88ee5da92fdae6bcd16a89319f1697efb9733",
	"A7_AMD64_ARCHIVE_SHA256":  "43018046ab37027b7fba3888d288961cb5abc136e478deaa9f878586bcce6629",
	"A7_ARM64_ARCHIVE_SHA256":  "e72653fd3094537caa60398e2017d409796d7ceef88a7662ca93b6299e9d00ec",
	"A7_AMD64_MANIFEST_SHA256": "3f7c879c93dea0d119805987bef434b65c1a53684e80f78b5d9a0c9c2cd011d5",
	"A7_ARM64_MANIFEST_SHA256": "87ef2a30d6ee8c9abe9e679d597d0a4fbe9bb5cdee1266572476ad6a66aef975",
}

func TestReleasePublicKeyReceipt_A8PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA8TagVersionGuard(scripts) {
		t.Fatal("A8 release is not conjunctively restricted to tag v0.50.79 and version 0.50.79")
	}
	for _, required := range []string{
		"release_phase='A8'", "prior_phase='A7'", `prior_tree="$A7_TREE_SHA"`,
		`'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A8 exact A7 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA8FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA8Tag, publicKeyReceiptA8Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchive(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A8 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.79"`)) {
			t.Fatalf("A8 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A8 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA7FixtureSealsA8DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA7Tag, publicKeyReceiptA7Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A7 to A8 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A8 exact A7 key record verified") {
			t.Fatalf("valid A8 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A7 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A7 annotated tag object differs from its pin", tamper: tamperA8TagObject},
		{name: "source_tree", code: "A7 source tree differs from its pin", tamper: tamperA8TreePin},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA8ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA8ManifestPin},
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
				t.Fatalf("tampered A8 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A8 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A8 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA8TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA8TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA8ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA8ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA8TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "79")
}

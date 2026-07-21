package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA11Tag     = "v0.50.82"
	publicKeyReceiptA11Version = "0.50.82"
)

var immutableA10LineagePins = map[string]string{
	"A10_COMMIT_SHA":            "54536edc09c37a634532c2c9b51e62869d393db4",
	"A10_TREE_SHA":              "e9a30f4530e06c9b62933e7bf97e0056faed259c",
	"A10_TAG_OBJECT_SHA":        "8b37fccb57255fc24003dc3af2700334f4a8d3c4",
	"A10_CHECKSUMS_SHA256":      "2e97c1f3c8d0cba0f93dd83c724c71eaa4966c79d4812a6a9cf034144c7b178d",
	"A10_AMD64_ARCHIVE_SHA256":  "b745eaddd8c70cb415aca42901213ffeb3c1d567f9b889e87a4a895ecfda8134",
	"A10_ARM64_ARCHIVE_SHA256":  "71a40ee709f34fb29bb562cde4587e2da1db1d6e8bc300d0edb4cfe63f8bec3c",
	"A10_AMD64_MANIFEST_SHA256": "98b38d8d59c5d146234e5a5f9bae26e80f8af0f699ac23e3f9fed5e59b32321e",
	"A10_ARM64_MANIFEST_SHA256": "976aa2bbeedd4e32b522373f6bf75a93b15f6813c4373c638c27d2cb98e4f00a",
}

func TestReleasePublicKeyReceipt_A11PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA11TagVersionGuard(scripts) {
		t.Fatal("A11 release is not conjunctively restricted to tag v0.50.82 and version 0.50.82")
	}
	for _, required := range []string{
		"release_phase='A11'", "prior_phase='A10'", `prior_tree="$A10_TREE_SHA"`,
		`'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A11 exact A10 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA11FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA11Tag, publicKeyReceiptA11Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchiveFile(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A11 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.82"`)) {
			t.Fatalf("A11 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A11 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA10FixtureSealsA11DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA10Tag, publicKeyReceiptA10Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A10 to A11 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A11 exact A10 key record verified") {
			t.Fatalf("valid A11 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A10 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A10 annotated tag object differs from its pin", tamper: tamperA11TagObject},
		{name: "source_tree", code: "A10 source tree differs from its pin", tamper: tamperA11TreePin},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA11ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA11ManifestPin},
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
				t.Fatalf("tampered A11 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A11 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A11 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA11TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA11TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA11ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA11ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA11TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "82")
}

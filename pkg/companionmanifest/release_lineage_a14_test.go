package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA14Tag     = "v0.50.85"
	publicKeyReceiptA14Version = "0.50.85"
)

var immutableA13LineagePins = map[string]string{
	"A13_COMMIT_SHA":            "2b7aa046bdb7861113dfa57b30489c11715582e9",
	"A13_TREE_SHA":              "95d1b00bcc1cb1bfcca3dd58e1e5e1b94575c367",
	"A13_TAG_OBJECT_SHA":        "de34e9c1a2a06b27f57235c81a59d1da180eab6d",
	"A13_CHECKSUMS_SHA256":      "8f00d3b42d71c9e71346bf62cd72f8e1428600cb0795f703d90de64b3b9ba14e",
	"A13_AMD64_ARCHIVE_SHA256":  "fa60e03ecd39a5fa203be3cca3e8a7010e3af7854195f0e866ef80e7a0e82f0f",
	"A13_ARM64_ARCHIVE_SHA256":  "f4ed0ef8d6f0274389ada5cebdeb87a2899bf34b7a11bd99318b5914775d84f1",
	"A13_AMD64_MANIFEST_SHA256": "ba6f3e92d4a1c0a1a52b7b17e484961cb8640944eae24856652ebe6192210931",
	"A13_ARM64_MANIFEST_SHA256": "22660fc029bbcb9ffe312964d9f674ba2587440dba48790e28fb4f35b19dcc69",
}

func TestReleasePublicKeyReceipt_A14PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA14TagVersionGuard(scripts) {
		t.Fatal("A14 release is not conjunctively restricted to tag v0.50.85 and version 0.50.85")
	}
	for _, required := range []string{
		"release_phase='A14'", "prior_phase='A13'", `prior_tree="$A13_TREE_SHA"`,
		`'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A14 exact A13 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA14FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA14Tag, publicKeyReceiptA14Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchiveFile(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A14 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.85"`)) {
			t.Fatalf("A14 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A14 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA13FixtureSealsA14DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA13Tag, publicKeyReceiptA13Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A13 to A14 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A14 exact A13 key record verified") {
			t.Fatalf("valid A14 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A13 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A13 annotated tag object differs from its pin", tamper: tamperA14TagObject},
		{name: "source_tree", code: "A13 source tree differs from its pin", tamper: tamperA14TreePin},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA14ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA14ManifestPin},
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
				t.Fatalf("tampered A14 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A14 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A14 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA14TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA14TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA14ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA14ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA14TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "85")
}

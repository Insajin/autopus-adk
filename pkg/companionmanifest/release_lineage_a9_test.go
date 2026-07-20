package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA9Tag     = "v0.50.80"
	publicKeyReceiptA9Version = "0.50.80"
)

var immutableA8LineagePins = map[string]string{
	"A8_COMMIT_SHA":            "dd0c2759ed5435d4634011e349caad62ea3df414",
	"A8_TREE_SHA":              "4325913ba332c583dd573ccf9248b38497d76926",
	"A8_TAG_OBJECT_SHA":        "8c6dcef91407e3321704014559cfd929d14768d0",
	"A8_CHECKSUMS_SHA256":      "1d0bdbfe50f85c381fde11c334c97a1b783dcfa4e12e0c4023152f38119a0bcd",
	"A8_AMD64_ARCHIVE_SHA256":  "19e317cdabc9dde976ca772d9ddbbf693b444dd44eefa70c8d0313a32de89a9b",
	"A8_ARM64_ARCHIVE_SHA256":  "41e29ae1c3c48dd6e3e5f4dfe8076472704d00a7d479b5cc8a90f53c0af6ef31",
	"A8_AMD64_MANIFEST_SHA256": "c5ac37874bac5de87152e781bd82a17c7705894f24be81657ccc907f15ba1f65",
	"A8_ARM64_MANIFEST_SHA256": "ebcf563c11f0836be2b2bd4423ea315283eeec12cfa200d479e1a56f5909f5f1",
}

func TestReleasePublicKeyReceipt_A9PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA9TagVersionGuard(scripts) {
		t.Fatal("A9 release is not conjunctively restricted to tag v0.50.80 and version 0.50.80")
	}
	for _, required := range []string{
		"release_phase='A9'", "prior_phase='A8'", `prior_tree="$A8_TREE_SHA"`,
		`'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A9 exact A8 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA9FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA9Tag, publicKeyReceiptA9Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchive(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A9 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.80"`)) {
			t.Fatalf("A9 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A9 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA8FixtureSealsA9DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA8Tag, publicKeyReceiptA8Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A8 to A9 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A9 exact A8 key record verified") {
			t.Fatalf("valid A9 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A8 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A8 annotated tag object differs from its pin", tamper: tamperA9TagObject},
		{name: "source_tree", code: "A8 source tree differs from its pin", tamper: tamperA9TreePin},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA9ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA9ManifestPin},
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
				t.Fatalf("tampered A9 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A9 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A9 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA9TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA9TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA9ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA9ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA9TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "80")
}

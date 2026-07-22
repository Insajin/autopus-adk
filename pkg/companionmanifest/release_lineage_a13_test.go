package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA13Tag     = "v0.50.84"
	publicKeyReceiptA13Version = "0.50.84"
)

var immutableA12LineagePins = map[string]string{
	"A12_COMMIT_SHA":            "e6367b5375cd4cdf09cb1515877bc57323521364",
	"A12_TREE_SHA":              "6c9a22e85d5a8c5f23c0d9e1bb41de270cab85a4",
	"A12_TAG_OBJECT_SHA":        "080507fceb3b4bf31f0e0887e49013fd65645ac2",
	"A12_CHECKSUMS_SHA256":      "7d871b077766f3a7dd6859427fa9b1333422312764820243d3bf7af5e935dee0",
	"A12_AMD64_ARCHIVE_SHA256":  "da92acfa4e8f45a0abea90b0991ae87cc7fb345c4f1ca2c166a8626670df658b",
	"A12_ARM64_ARCHIVE_SHA256":  "5b29fdb21b62f8933c1ff0608f9c1dca096be24649fd24ec40bcbe9ff72c4fcc",
	"A12_AMD64_MANIFEST_SHA256": "caa1145bc293a125495795914005429694e2a2b98a863d903a40575495ec250a",
	"A12_ARM64_MANIFEST_SHA256": "013e7b98bfea64783d932e787609d526d5157801788b90b13cc59990070ab20b",
}

func TestReleasePublicKeyReceipt_A13PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA13TagVersionGuard(scripts) {
		t.Fatal("A13 release is not conjunctively restricted to tag v0.50.84 and version 0.50.84")
	}
	for _, required := range []string{
		"release_phase='A13'", "prior_phase='A12'", `prior_tree="$A12_TREE_SHA"`,
		`'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A13 exact A12 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA13FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA13Tag, publicKeyReceiptA13Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchiveFile(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A13 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.84"`)) {
			t.Fatalf("A13 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A13 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA12FixtureSealsA13DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA12Tag, publicKeyReceiptA12Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A12 to A13 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A13 exact A12 key record verified") {
			t.Fatalf("valid A13 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A12 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A12 annotated tag object differs from its pin", tamper: tamperA13TagObject},
		{name: "source_tree", code: "A12 source tree differs from its pin", tamper: tamperA13TreePin},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA13ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA13ManifestPin},
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
				t.Fatalf("tampered A13 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A13 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A13 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA13TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA13TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA13ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA13ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA13TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "84")
}

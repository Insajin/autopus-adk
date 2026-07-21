package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA7Tag     = "v0.50.78"
	publicKeyReceiptA7Version = "0.50.78"
)

var immutableA6LineagePins = map[string]string{
	"A6_COMMIT_SHA":            "902f1acfa91f1d0a2ac9471d5cd79117031a2599",
	"A6_TAG_OBJECT_SHA":        "41feed7decafac33d8f7f43e06804e3c9bf37ef3",
	"A6_CHECKSUMS_SHA256":      "fb1a35dcdb44255aad43b7ae74950ed59f05ccf44abde9cadf28ecfa0dfce37a",
	"A6_AMD64_ARCHIVE_SHA256":  "d5e47076c1fc898d2b3f5880b6edfcf9a12e805633dcba2691da22f300d41dc9",
	"A6_ARM64_ARCHIVE_SHA256":  "d6d092177a5406c194eea1de4fbd11b8af92a03814eb143a294541a3a578b9ab",
	"A6_AMD64_MANIFEST_SHA256": "64c634130b16a74cbb33f666d316a05d9a7a1012246dc58fde6e15350b71d0c5",
	"A6_ARM64_MANIFEST_SHA256": "b6611c04990b048bc5545e37c942bc8e7e4fab8592d546eaab80d7084991bea6",
}

func TestReleasePublicKeyReceipt_A7PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA7TagVersionGuard(scripts) {
		t.Fatal("A7 release is not conjunctively restricted to tag v0.50.78 and version 0.50.78")
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA7FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA7Tag, publicKeyReceiptA7Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchiveFile(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A7 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.78"`)) {
			t.Fatalf("A7 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A7 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA6FixtureSealsA7DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA6Tag, publicKeyReceiptA6Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A6 to A7 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A7 exact A6 key record verified") {
			t.Fatalf("valid A7 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A6 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A6 annotated tag object differs from its pin", tamper: tamperA7TagObject},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA7ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA7ManifestPin},
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
				t.Fatalf("tampered A7 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A7 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A7 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA7TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA7ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA7ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA7TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "78")
}

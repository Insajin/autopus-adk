package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA5Tag     = "v0.50.74"
	publicKeyReceiptA5Version = "0.50.74"
)

var immutableA4LineagePins = map[string]string{
	"A4_COMMIT_SHA":            "334b297f05942accbecdfa15b54e38e005c82f2d",
	"A4_TAG_OBJECT_SHA":        "b1ebab0af82536f8a4bc1ed93f31f82f6c53d008",
	"A4_CHECKSUMS_SHA256":      "a30e0893f1565919e9e90dd7e1f2b19e5487024b0373f66de56729e1d747e7d1",
	"A4_AMD64_ARCHIVE_SHA256":  "da7f6ef4396591ff0b728f976536d261ecb084038fffab7c7662a6f7329ade2a",
	"A4_ARM64_ARCHIVE_SHA256":  "ff046f6af316236166d514608a1b432c2f3a01efbd8aab03b54d2c2639d2f422",
	"A4_AMD64_MANIFEST_SHA256": "86940b9c7eb89308aff4260d9a6178d933d3f1a9833e601ac8c1e914c225a7b5",
	"A4_ARM64_MANIFEST_SHA256": "a68a10a46b0778ccc858855323fd45cf0b9727f76fa45b16efdbc83b320128f0",
}

func TestReleasePublicKeyReceipt_GoReleaserA5FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA5Tag, publicKeyReceiptA5Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchive(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A5 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.74"`)) {
			t.Fatalf("A5 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A5 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA4FixtureSealsA5DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA4Tag, publicKeyReceiptA4Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A4 to A5 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A5 exact A4 key record verified") {
			t.Fatalf("valid A5 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A4 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A4 annotated tag object differs from its pin", tamper: tamperA5TagObject},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA5ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA5ManifestPin},
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
				t.Fatalf("tampered A5 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A5 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A5 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA5TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA5ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA5ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA4TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "73")
}

func exactA5TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "74")
}

func exactLineageTagVersionGuard(source, patch string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`GITHUB_REF_NAME.{0,240}v0\.50\.` + patch + `.{0,400}COMPANION_VERSION.{0,240}0\.50\.` + patch),
		regexp.MustCompile(`COMPANION_VERSION.{0,240}0\.50\.` + patch + `.{0,400}GITHUB_REF_NAME.{0,240}v0\.50\.` + patch),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(source) {
			return true
		}
	}
	return false
}

package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA6Tag     = "v0.50.75"
	publicKeyReceiptA6Version = "0.50.75"
)

var immutableA5LineagePins = map[string]string{
	"A5_COMMIT_SHA":            "b27252cb1148192a8ae1a95195c50e5f221453a4",
	"A5_TAG_OBJECT_SHA":        "c79f133f0108bf3f07cee0162c1abeecf9d379d1",
	"A5_CHECKSUMS_SHA256":      "48c79e1fb47444aa83909794cd041bdfed18bf263bf5c0209578540382824ad4",
	"A5_AMD64_ARCHIVE_SHA256":  "aeb9d048579c77ab17f4a4ec3a1160778d16c627747c5af5f341e664e1417cb0",
	"A5_ARM64_ARCHIVE_SHA256":  "bc90e594c91de61dabc2982f60249b638d448fa3f6643004fe6d45cdd0cc5eab",
	"A5_AMD64_MANIFEST_SHA256": "5b4381d3f2180b19c0da9d419ebc8452b9ba04c73c8d0921c2a74c09ab38b85c",
	"A5_ARM64_MANIFEST_SHA256": "62a9f78302ee000c16c1c73669282e955fc3abc82f850ff4a77d0e04069f4aed",
}

func TestReleasePublicKeyReceipt_GoReleaserA6FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA6Tag, publicKeyReceiptA6Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchive(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A6 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.75"`)) {
			t.Fatalf("A6 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A6 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA5FixtureSealsA6DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA5Tag, publicKeyReceiptA5Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A5 to A6 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A6 exact A5 key record verified") {
			t.Fatalf("valid A6 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A5 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A5 annotated tag object differs from its pin", tamper: tamperA6TagObject},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA6ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA6ManifestPin},
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
				t.Fatalf("tampered A6 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A6 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A6 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA6TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA6ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA6ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA6TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "75")
}

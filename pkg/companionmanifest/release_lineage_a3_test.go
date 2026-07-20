package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestReleasePublicKeyReceipt_GoReleaserA3FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA3Tag, publicKeyReceiptA3Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchive(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A3 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.72"`)) {
			t.Fatalf("A3 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A3 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA2FixtureSealsA3DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA2Tag, publicKeyReceiptA2Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A2 to A3 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A3 exact A2 key record verified") {
			t.Fatalf("valid A3 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A2 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A2 annotated tag object differs from its pin", tamper: tamperA3TagObject},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA3ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA3ManifestPin},
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
				t.Fatalf("tampered A3 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A3 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A3 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func directPredecessorPinReplacements(fixture *executableLineageFixture) map[string]string {
	prefix := ""
	switch fixture.currentTag {
	case publicKeyReceiptA2Tag:
		prefix = "A1"
	case publicKeyReceiptA3Tag:
		prefix = "A2"
	case publicKeyReceiptA4Tag:
		prefix = "A3"
	case publicKeyReceiptA5Tag:
		prefix = "A4"
	case publicKeyReceiptA6Tag:
		prefix = "A5"
	case publicKeyReceiptA7Tag:
		prefix = "A6"
	case publicKeyReceiptA8Tag:
		prefix = "A7"
	default:
		return nil
	}
	replacements := map[string]string{
		prefix + "_COMMIT_SHA":            fixture.pins.commit,
		prefix + "_TAG_OBJECT_SHA":        fixture.pins.tagObject,
		prefix + "_CHECKSUMS_SHA256":      fixture.pins.checksums,
		prefix + "_AMD64_ARCHIVE_SHA256":  fixture.pins.amd64Archive,
		prefix + "_ARM64_ARCHIVE_SHA256":  fixture.pins.arm64Archive,
		prefix + "_AMD64_MANIFEST_SHA256": fixture.pins.amd64Manifest,
		prefix + "_ARM64_MANIFEST_SHA256": fixture.pins.arm64Manifest,
	}
	if prefix == "A7" {
		replacements[prefix+"_TREE_SHA"] = fixture.pins.tree
	}
	return replacements
}

func immutableProductionLineagePin(name string) (string, bool) {
	for _, pins := range []map[string]string{
		immutableA0LineagePins, immutableA1LineagePins, immutableA2LineagePins,
		immutableA3LineagePins, immutableA4LineagePins, immutableA5LineagePins,
		immutableA6LineagePins, immutableA7LineagePins,
	} {
		if value, ok := pins[name]; ok {
			return value, true
		}
	}
	return "", false
}

func tamperA3TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA3ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA3ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

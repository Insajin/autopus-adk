package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestReleasePublicKeyReceipt_GoReleaserA2FixtureProducesCurrentArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA2Tag, publicKeyReceiptA2Version, true,
	)
	for _, architecture := range []string{"amd64", "arm64"} {
		entries, err := decodeLineageArchiveFile(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode A2 %s archive: %v", architecture, err)
		}
		manifest := entries["adk-companion-manifest.json"].data
		if !bytes.Contains(manifest, []byte(`"version":"0.50.71"`)) {
			t.Fatalf("A2 %s manifest does not carry the current version", architecture)
		}
		for _, name := range []string{
			lineageBundleName + "/public-key-receipt.json",
			lineageBundleName + "/public-key-receipt.sig",
		} {
			if len(entries[name].data) == 0 {
				t.Fatalf("A2 %s archive is missing %s", architecture, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA1FixtureSealsA2DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserA1FixtureEvidence(t, tools)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A1 to A2 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A2 exact A1 key record verified") {
			t.Fatalf("valid A2 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A1 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A1 annotated tag object differs from its pin", tamper: tamperA2TagObject},
		{name: "archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA2ArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA2ManifestPin},
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
				t.Fatalf("tampered A2 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A2 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A2 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA2TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.tagObject = differentHex(fixture.tagObject, 40)
	fixture.writeEvidence(t)
}

func tamperA2ArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA2ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

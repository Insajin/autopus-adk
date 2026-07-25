package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA18Tag     = "v0.50.89"
	publicKeyReceiptA18Version = "0.50.89"
)

var immutableA17LineagePins = map[string]string{
	"A17_RELEASE_ID":                 "359675749",
	"A17_COMMIT_SHA":                 "2b062a5e348fbecc414abe9ba5c74c7dc79fe243",
	"A17_TREE_SHA":                   "1e376104c94748a546607abce6667813163d3d61",
	"A17_TAG_OBJECT_SHA":             "8721b6be61a058aa5987727d3bae6d87c5fb38f4",
	"A17_CHECKSUMS_SHA256":           "a4b437c11a5aaad986a4e0b93b0db0e19c3a49a1a801ba25e8b096c462bf3c47",
	"A17_AMD64_ARCHIVE_SHA256":       "3bae06adc5c31281efd8d9fd016af0ab9158da54262e7f17fafd2ed9be6ff1b1",
	"A17_ARM64_ARCHIVE_SHA256":       "bf7a96a4ce34b58a940ab9813c2b797ec8f4a489c794b54d6b54097c5ecc4cce",
	"A17_LINUX_AMD64_ARCHIVE_SHA256": "e09b66b7a1683ef8764e59ca620242ee7eebbee573c0fec5232f9adb649f6909",
	"A17_LINUX_ARM64_ARCHIVE_SHA256": "31cc659ae347346204db2dae13368e6277b1e21f7e3a7c16dab160dd2b98176d",
	"A17_AMD64_MANIFEST_SHA256":      "56b4b53840ee7c859077245ff6ddc95ef0ea530f581b3831aa6ee8645b9a9749",
	"A17_ARM64_MANIFEST_SHA256":      "457b6fef8ebcda7b3977a0786f2397a30591225ac5112858aab5ba920176df8c",
}

func TestReleasePublicKeyReceipt_A18PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA18TagVersionGuard(scripts) {
		t.Fatal("A18 release is not conjunctively restricted to tag v0.50.89 and version 0.50.89")
	}
	for _, required := range []string{
		"release_phase='A18'", "prior_phase='A17'",
		`prior_release_id="$A17_RELEASE_ID"`, `prior_tree="$A17_TREE_SHA"`,
		`prior_linux_amd64_archive="$A17_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A17_LINUX_ARM64_ARCHIVE_SHA256"`,
		"verify-public-key-lineage-assets.sh", `'.commit.tree.sha'`, `'.id'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A18 exact A17 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA18FixtureProducesPlatformArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA18Tag, publicKeyReceiptA18Version, true,
	)
	for _, target := range executableLineageArchiveTargets {
		entries, err := decodeLineageArchiveFile(evidence.archives[target.key])
		if err != nil {
			t.Fatalf("decode A18 %s archive: %v", target.key, err)
		}
		if target.platform == "darwin" {
			manifest := entries["adk-companion-manifest.json"].data
			if !bytes.Contains(manifest, []byte(`"version":"0.50.89"`)) {
				t.Fatalf("A18 %s manifest does not carry the current version", target.key)
			}
			for _, name := range darwinLineageArchiveEvidenceNames {
				if len(entries[name].data) == 0 {
					t.Fatalf("A18 %s archive is missing %s", target.key, name)
				}
			}
			continue
		}
		for _, name := range darwinLineageArchiveEvidenceNames {
			if _, exists := entries[name]; exists {
				t.Fatalf("A18 %s Linux archive unexpectedly contains %s", target.key, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA17FixtureSealsA18DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA17Tag, publicKeyReceiptA17Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A17 to A18 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A18 exact A17 key record verified") {
			t.Fatalf("valid A18 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A17 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "release_id", code: "A17 release ID differs from its pin", tamper: tamperA18ReleaseID},
		{name: "annotated_tag_object", code: "A17 annotated tag object differs from its pin", tamper: tamperA18TagObject},
		{name: "source_tree", code: "A17 source tree differs from its pin", tamper: tamperA18TreePin},
		{name: "archive_api_digest", code: "server digest differs", tamper: tamperA18ArchiveAPIDigest},
		{name: "darwin_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperLineageArchiveBytes},
		{name: "darwin_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA18DarwinArchivePin},
		{name: "linux_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperA18LinuxArchiveBytes},
		{name: "linux_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA18LinuxArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA18ManifestPin},
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
				t.Fatalf("tampered A18 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A18 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A18 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA18ReleaseID(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.releaseID = "2"
}

func tamperA18TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA18TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA18ArchiveAPIDigest(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.assetDigestOverride = "sha256:" + differentHex(fixture.pins.amd64Archive, 64)
	fixture.writeEvidence(t)
}

func tamperA18DarwinArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA18LinuxArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.linuxAMD64Archive = differentHex(fixture.pins.linuxAMD64Archive, 64)
}

func tamperA18LinuxArchiveBytes(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.archiveMutation = &lineageArchiveMutation{
		architecture: "linux_amd64", entry: "README.md",
		mutate: func(_ *testing.T, data []byte) ([]byte, bool) {
			return append(data, '\n'), true
		},
	}
	fixture.writeEvidence(t)
}

func tamperA18ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA18TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "89")
}

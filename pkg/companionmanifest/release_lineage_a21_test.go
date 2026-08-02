package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA21Tag     = "v0.50.92"
	publicKeyReceiptA21Version = "0.50.92"
)

var immutableA20LineagePins = map[string]string{
	"A20_RELEASE_ID":                 "360327495",
	"A20_COMMIT_SHA":                 "7f44e4f143b2348c02553bab2209088c966f81ae",
	"A20_TREE_SHA":                   "8fdf3615e40f5e81512517619d4225ee067f8c23",
	"A20_TAG_OBJECT_SHA":             "60158446cbf0599317b2850609d5e0d957a965db",
	"A20_CHECKSUMS_SHA256":           "ff546eed918bc35dc420451341da563f2d6487c3a6565f9058dbc99362943f5a",
	"A20_AMD64_ARCHIVE_SHA256":       "97a9fc7c6da9348709e3dbc48cb8587551e9980d61a33566103c844abdd3f05c",
	"A20_ARM64_ARCHIVE_SHA256":       "baff6e099c8debc97bdf61bf0ea98813e2bcdef58f5171cad67edb84bd64b548",
	"A20_LINUX_AMD64_ARCHIVE_SHA256": "1011d88f66658f2b9ebe2caefb536038266005f229f846de2f1e597b17e25ea6",
	"A20_LINUX_ARM64_ARCHIVE_SHA256": "2905bc851e4637c60fd198f42331a7a6bf13180ee917ca67d9f20affce436c5d",
	"A20_AMD64_MANIFEST_SHA256":      "d34a841d2dbca009e7ae7af2780ed8792e2501b04f30b0711d197093ea61a24c",
	"A20_ARM64_MANIFEST_SHA256":      "2c08f77641786a8afe9e94de8734cb7a318868d3189e336ef01b704cca0e40d0",
}

func TestReleasePublicKeyReceipt_A21PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA21TagVersionGuard(scripts) {
		t.Fatal("A21 release is not conjunctively restricted to tag v0.50.92 and version 0.50.92")
	}
	for _, required := range []string{
		"release_phase='A21'", "prior_phase='A20'",
		`prior_release_id="$A20_RELEASE_ID"`, `prior_tree="$A20_TREE_SHA"`,
		`prior_linux_amd64_archive="$A20_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A20_LINUX_ARM64_ARCHIVE_SHA256"`,
		"verify-public-key-lineage-assets.sh", `'.commit.tree.sha'`, `'.id'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A21 exact A20 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA21FixtureProducesPlatformArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA21Tag, publicKeyReceiptA21Version, true,
	)
	for _, target := range executableLineageArchiveTargets {
		entries, err := decodeLineageArchiveFile(evidence.archives[target.key])
		if err != nil {
			t.Fatalf("decode A21 %s archive: %v", target.key, err)
		}
		if target.platform == "darwin" {
			manifest := entries["adk-companion-manifest.json"].data
			if !bytes.Contains(manifest, []byte(`"version":"0.50.92"`)) {
				t.Fatalf("A21 %s manifest does not carry the current version", target.key)
			}
			for _, name := range darwinLineageArchiveEvidenceNames {
				if len(entries[name].data) == 0 {
					t.Fatalf("A21 %s archive is missing %s", target.key, name)
				}
			}
			continue
		}
		for _, name := range darwinLineageArchiveEvidenceNames {
			if _, exists := entries[name]; exists {
				t.Fatalf("A21 %s Linux archive unexpectedly contains %s", target.key, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA20FixtureSealsA21DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA20Tag, publicKeyReceiptA20Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A20 to A21 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A21 exact A20 key record verified") {
			t.Fatalf("valid A21 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A20 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "release_id", code: "A20 release ID differs from its pin", tamper: tamperA21ReleaseID},
		{name: "annotated_tag_object", code: "A20 annotated tag object differs from its pin", tamper: tamperA21TagObject},
		{name: "source_tree", code: "A20 source tree differs from its pin", tamper: tamperA21TreePin},
		{name: "archive_api_digest", code: "server digest differs", tamper: tamperA21ArchiveAPIDigest},
		{name: "darwin_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperLineageArchiveBytes},
		{name: "darwin_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA21DarwinArchivePin},
		{name: "linux_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperA21LinuxArchiveBytes},
		{name: "linux_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA21LinuxArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA21ManifestPin},
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
				t.Fatalf("tampered A21 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A21 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A21 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA21ReleaseID(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.releaseID = "2"
}

func tamperA21TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA21TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA21ArchiveAPIDigest(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.assetDigestOverride = "sha256:" + differentHex(fixture.pins.amd64Archive, 64)
	fixture.writeEvidence(t)
}

func tamperA21DarwinArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA21LinuxArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.linuxAMD64Archive = differentHex(fixture.pins.linuxAMD64Archive, 64)
}

func tamperA21LinuxArchiveBytes(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.archiveMutation = &lineageArchiveMutation{
		architecture: "linux_amd64", entry: "README.md",
		mutate: func(_ *testing.T, data []byte) ([]byte, bool) {
			return append(data, '\n'), true
		},
	}
	fixture.writeEvidence(t)
}

func tamperA21ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA21TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "92")
}

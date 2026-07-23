package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA16Tag     = "v0.50.87"
	publicKeyReceiptA16Version = "0.50.87"
)

var immutableA15LineagePins = map[string]string{
	"A15_COMMIT_SHA":                 "0fc4f60dac8ff8afe69b680c8bf723bfbced4769",
	"A15_TREE_SHA":                   "3daa4aef3528338439acb34f50d3b4a19ababea5",
	"A15_TAG_OBJECT_SHA":             "bb24ad6a554beee871063070b219b409245c0e93",
	"A15_CHECKSUMS_SHA256":           "237f985675f866c234a41066735a2bff3ae0b554a2fe1b1b6b57aed125bac8f7",
	"A15_AMD64_ARCHIVE_SHA256":       "41e2a371c89567ff862d5f47179c838cb3aefd83abeb0ff769e58b12579676e3",
	"A15_ARM64_ARCHIVE_SHA256":       "84ea326a10c860af82663db1c87a8a15bdee492143d77a02ad86a0b3ba930f8f",
	"A15_LINUX_AMD64_ARCHIVE_SHA256": "cae69dd8828cb2c12ba0d312c3f4dbc034104c1b4b9cee6cddf18eebe6430cb6",
	"A15_LINUX_ARM64_ARCHIVE_SHA256": "9e943908dabf910e9f3072f838a99dec3c9d4952d9058bfbc1b71cd78e3f29eb",
	"A15_AMD64_MANIFEST_SHA256":      "c2398cd51093cb19804ef2d07e1848cc77d16610a4669e78e0e1577a466df300",
	"A15_ARM64_MANIFEST_SHA256":      "83da7620c878841c06980ad12315023dc2054b71f27cb7dfb53931a4224d0099",
}

func TestReleasePublicKeyReceipt_A16PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA16TagVersionGuard(scripts) {
		t.Fatal("A16 release is not conjunctively restricted to tag v0.50.87 and version 0.50.87")
	}
	for _, required := range []string{
		"release_phase='A16'", "prior_phase='A15'", `prior_tree="$A15_TREE_SHA"`,
		`prior_linux_amd64_archive="$A15_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A15_LINUX_ARM64_ARCHIVE_SHA256"`,
		"verify-public-key-lineage-assets.sh", `'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A16 exact A15 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA16FixtureProducesPlatformArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA16Tag, publicKeyReceiptA16Version, true,
	)
	for _, target := range executableLineageArchiveTargets {
		entries, err := decodeLineageArchiveFile(evidence.archives[target.key])
		if err != nil {
			t.Fatalf("decode A16 %s archive: %v", target.key, err)
		}
		if target.platform == "darwin" {
			manifest := entries["adk-companion-manifest.json"].data
			if !bytes.Contains(manifest, []byte(`"version":"0.50.87"`)) {
				t.Fatalf("A16 %s manifest does not carry the current version", target.key)
			}
			for _, name := range darwinLineageArchiveEvidenceNames {
				if len(entries[name].data) == 0 {
					t.Fatalf("A16 %s archive is missing %s", target.key, name)
				}
			}
			continue
		}
		for _, name := range darwinLineageArchiveEvidenceNames {
			if _, exists := entries[name]; exists {
				t.Fatalf("A16 %s Linux archive unexpectedly contains %s", target.key, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA15FixtureSealsA16DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA15Tag, publicKeyReceiptA15Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A15 to A16 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A16 exact A15 key record verified") {
			t.Fatalf("valid A16 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A15 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A15 annotated tag object differs from its pin", tamper: tamperA16TagObject},
		{name: "source_tree", code: "A15 source tree differs from its pin", tamper: tamperA16TreePin},
		{name: "archive_api_digest", code: "server digest differs", tamper: tamperA16ArchiveAPIDigest},
		{name: "darwin_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperLineageArchiveBytes},
		{name: "darwin_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA16DarwinArchivePin},
		{name: "linux_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperA16LinuxArchiveBytes},
		{name: "linux_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA16LinuxArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA16ManifestPin},
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
				t.Fatalf("tampered A16 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A16 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A16 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA16TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA16TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA16ArchiveAPIDigest(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.assetDigestOverride = "sha256:" + differentHex(fixture.pins.amd64Archive, 64)
	fixture.writeEvidence(t)
}

func tamperA16DarwinArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA16LinuxArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.linuxAMD64Archive = differentHex(fixture.pins.linuxAMD64Archive, 64)
}

func tamperA16LinuxArchiveBytes(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.archiveMutation = &lineageArchiveMutation{
		architecture: "linux_amd64", entry: "README.md",
		mutate: func(_ *testing.T, data []byte) ([]byte, bool) {
			return append(data, '\n'), true
		},
	}
	fixture.writeEvidence(t)
}

func tamperA16ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA16TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "87")
}

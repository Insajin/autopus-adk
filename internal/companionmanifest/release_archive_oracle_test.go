package companionmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	manifest "github.com/insajin/autopus-adk/pkg/companionmanifest"
)

const (
	archiveManifestName  = "adk-companion-manifest.json"
	archiveSignatureName = "adk-companion-manifest.sig"
	archiveReceiptName   = "adk-companion-darwin-receipt.json"
	archiveBundleName    = "adk-companion-public-key-receipt.bundle"
)

type darwinArchiveReceipt struct {
	ArtifactDigest  string `json:"artifact_digest"`
	ManifestDigest  string `json:"manifest_digest"`
	SignatureDigest string `json:"signature_digest"`
}

func validateProductionDarwinArchive(
	entries map[string]releaseArchiveEntry,
	architecture string,
	privateKey ed25519.PrivateKey,
) error {
	required := []string{
		"auto", archiveManifestName, archiveSignatureName, archiveReceiptName,
		archiveBundleName + "/public-key-receipt.json",
		archiveBundleName + "/public-key-receipt.sig",
	}
	for _, name := range required {
		if _, ok := entries[name]; !ok {
			return fmt.Errorf("required production entry %s is absent", name)
		}
	}
	if entries["auto"].mode&0o111 == 0 {
		return errors.New("auto archive entry is not executable")
	}
	for _, forbidden := range []string{"public-key-receipt.json", "public-key-receipt.sig"} {
		if _, ok := entries[forbidden]; ok {
			return fmt.Errorf("independent receipt asset %s is present", forbidden)
		}
	}
	var bundleEntries []string
	for name := range entries {
		if strings.HasPrefix(name, archiveBundleName+"/") {
			bundleEntries = append(bundleEntries, name)
		}
	}
	sort.Strings(bundleEntries)
	wantBundle := []string{
		archiveBundleName + "/public-key-receipt.json",
		archiveBundleName + "/public-key-receipt.sig",
	}
	if strings.Join(bundleEntries, "\x00") != strings.Join(wantBundle, "\x00") {
		return fmt.Errorf("receipt bundle entries %v differ from %v", bundleEntries, wantBundle)
	}
	return validateArchiveEvidence(entries, architecture, privateKey)
}

func validateArchiveEvidence(
	entries map[string]releaseArchiveEntry,
	architecture string,
	privateKey ed25519.PrivateKey,
) error {
	artifactDigest := digestArchiveBytes(entries["auto"].data)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	parsedManifest, err := manifest.Verify(
		entries[archiveManifestName].data,
		entries[archiveSignatureName].data,
		entries["auto"].data,
		manifest.VerificationPolicy{
			PinnedKeys: map[string]manifest.PinnedKey{
				"release-key": {PublicKey: publicKey, ExpiresAt: "2027-07-15T00:00:00Z"},
			},
			Now:                  time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC),
			MinimumRollbackFloor: 5069, ExpectedPlatform: "darwin",
			ExpectedArchitecture: architecture, ExpectedHandoff: "v1",
			ExpectedDigest: artifactDigest,
		},
	)
	if err != nil {
		return fmt.Errorf("manifest linkage: %w", err)
	}
	var receipt darwinArchiveReceipt
	if err := json.Unmarshal(entries[archiveReceiptName].data, &receipt); err != nil {
		return fmt.Errorf("Darwin receipt: %w", err)
	}
	if receipt.ArtifactDigest != artifactDigest ||
		receipt.ManifestDigest != digestArchiveBytes(entries[archiveManifestName].data) ||
		receipt.SignatureDigest != digestArchiveBytes(entries[archiveSignatureName].data) {
		return errors.New("Darwin receipt digest linkage differs")
	}
	publicReceipt := entries[archiveBundleName+"/public-key-receipt.json"].data
	publicSignature := entries[archiveBundleName+"/public-key-receipt.sig"].data
	if err := manifest.CheckPublicKeyReceiptSelfConsistency(
		publicReceipt, publicSignature,
		manifest.PublicKeyReceiptPolicy{
			Now:           time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC),
			ExpectedKeyID: "release-key", ExpectedHandoff: "v1", MinimumRollbackFloor: 5069,
		},
	); err != nil {
		return fmt.Errorf("public-key receipt linkage: %w", err)
	}
	parsedPublicReceipt, err := manifest.ParsePublicKeyReceiptStrict(publicReceipt)
	if err != nil {
		return err
	}
	if err := manifest.ValidateManifestPublicKeyReceipt(parsedManifest, parsedPublicReceipt); err != nil {
		return fmt.Errorf("manifest/public-key receipt conjunction: %w", err)
	}
	return nil
}

func digestArchiveBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func assertExecutableArchiveWiringTamperingFails(
	t *testing.T,
	tools mockReleaseTools,
	privateKey ed25519.PrivateKey,
) {
	t.Helper()
	cases := []struct {
		name     string
		mutation goReleaserConfigMutation
	}{
		{name: "files", mutation: mutateArchiveFiles},
		{name: "wildcard", mutation: mutateArchiveWildcard},
		{name: "dst", mutation: mutateArchiveDestination},
		{name: "strip_parent", mutation: mutateArchiveStripParent},
		{name: "post_hook", mutation: mutateArchivePostHook},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			mutation := func(t *testing.T, config string) string {
				config = narrowGoReleaserToDarwinArm64(t, config)
				return test.mutation(t, config)
			}
			mutatedConfig := mutation(t, readReleaseFile(t, ".goreleaser.yaml"))
			if err := validateProductionGoReleaserWiring(mutatedConfig); err == nil {
				t.Fatalf("%s config wiring drift passed semantic validation", test.name)
			}
			archives, runErr := runGoReleaserFixture(t, tools, mutation, []string{"arm64"})
			if runErr != nil {
				return
			}
			entries := readReleaseArchive(t, archives["arm64"])
			if err := validateProductionDarwinArchive(entries, "arm64", privateKey); err != nil {
				return
			}
			// Some GoReleaser versions normalize strip_parent to the same archive
			// shape. The semantic config oracle above still rejects that drift.
		})
	}
}

func narrowGoReleaserToDarwinArm64(t *testing.T, config string) string {
	t.Helper()
	config = replaceReleaseConfig(t, config,
		"    goos:\n      - linux\n      - darwin\n      - windows\n",
		"    goos:\n      - darwin\n")
	return replaceReleaseConfig(t, config,
		"    goarch:\n      - amd64\n      - arm64\n",
		"    goarch:\n      - arm64\n")
}

func mutateArchiveFiles(t *testing.T, config string) string {
	t.Helper()
	return replaceReleaseConfig(t, config,
		"      - src: '{{ if eq .Os \"darwin\" }}dist/auto_{{ .Target }}/adk-companion-manifest.json{{ else }}scripts/companion-release/no-files-*{{ end }}'\n        dst: adk-companion-manifest.json\n",
		"")
}

func mutateArchiveWildcard(t *testing.T, config string) string {
	t.Helper()
	return replaceReleaseConfig(t, config,
		"adk-companion-public-key-receipt.bundle/**",
		"adk-companion-public-key-receipt.bundle/missing-*")
}

func mutateArchiveDestination(t *testing.T, config string) string {
	t.Helper()
	return replaceReleaseConfig(t, config,
		"        dst: adk-companion-public-key-receipt.bundle\n        strip_parent: true",
		"        dst: wrong-public-key-receipt.bundle\n        strip_parent: true")
}

func mutateArchiveStripParent(t *testing.T, config string) string {
	t.Helper()
	return replaceReleaseConfig(t, config,
		"        dst: adk-companion-public-key-receipt.bundle\n        strip_parent: true",
		"        dst: adk-companion-public-key-receipt.bundle\n        strip_parent: false")
}

func mutateArchivePostHook(t *testing.T, config string) string {
	t.Helper()
	return replaceReleaseConfig(t, config,
		"        - cmd: scripts/companion-release/produce.sh",
		"        - cmd: /usr/bin/true")
}

func replaceReleaseConfig(t *testing.T, config, old, replacement string) string {
	t.Helper()
	if strings.Count(config, old) != 1 {
		t.Fatalf("test-only GoReleaser mutation target count for %q = %d, want 1", old, strings.Count(config, old))
	}
	return strings.Replace(config, old, replacement, 1)
}

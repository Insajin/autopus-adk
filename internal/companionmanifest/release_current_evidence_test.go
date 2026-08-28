package companionmanifest

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestCurrentReleaseVerifier_AcceptsExactImmutableRelease(t *testing.T) {
	fixture := newCurrentReleaseFixture(t)
	output, err := fixture.run()
	if err != nil {
		t.Fatalf("exact release rejected: %v\n%s", err, output)
	}
	got, err := os.ReadFile(fixture.output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fixture.checksums) {
		t.Fatal("materialized checksums differ from server bytes")
	}
	assertCurrentReleaseSignatureLog(t, fixture.signatureLog)
	assertCurrentReleaseVerifierLog(t, fixture.verifierLog)
}

func TestCurrentReleaseVerifier_RejectsUntrustedReleaseEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*currentReleaseFixture)
		want   string
	}{
		{name: "mutable_release", mutate: func(f *currentReleaseFixture) {
			f.release.Immutable = false
		}, want: "not exact, immutable, and complete"},
		{name: "partial_asset_set", mutate: func(f *currentReleaseFixture) {
			f.release.Assets = f.release.Assets[:len(f.release.Assets)-1]
		}, want: "not exact, immutable, and complete"},
		{name: "duplicate_asset", mutate: func(f *currentReleaseFixture) {
			f.release.Assets[len(f.release.Assets)-1].Name = f.release.Assets[0].Name
		}, want: "not exact, immutable, and complete"},
		{name: "checksums_server_digest_mismatch", mutate: func(f *currentReleaseFixture) {
			f.asset("checksums.txt").Digest = "sha256:" + strings.Repeat("f", 64)
		}, want: "differs from GitHub metadata"},
		{name: "bundle_server_digest_mismatch", mutate: func(f *currentReleaseFixture) {
			f.asset("checksums.txt.bundle").Digest = "sha256:" + strings.Repeat("f", 64)
		}, want: "differs from GitHub metadata"},
		{name: "envelope_server_digest_mismatch", mutate: func(f *currentReleaseFixture) {
			f.asset("checksums.txt.signatures").Digest = "sha256:" + strings.Repeat("f", 64)
		}, want: "differs from GitHub metadata"},
		{name: "cask_archive_checksum_mismatch", mutate: func(f *currentReleaseFixture) {
			f.asset(currentReleaseArchives[0]).Digest = "sha256:" + strings.Repeat("e", 64)
		}, want: "checksums.txt differs from GitHub API digest"},
		{name: "windows_archive_checksum_mismatch", mutate: func(f *currentReleaseFixture) {
			f.asset(currentReleaseArchives[7]).Digest = "sha256:" + strings.Repeat("d", 64)
		}, want: "checksums.txt differs from GitHub API digest"},
		{name: "bridge_manifest_active_claim", mutate: func(f *currentReleaseFixture) {
			f.replaceAsset("omp-context-bridge-release.v1.json",
				[]byte(`{"release_mode":"active"}`+"\n"))
		}, want: "released bridge manifest is not canonical-full"},
		{name: "rotation_document_differs_from_bridge_manifest", mutate: func(f *currentReleaseFixture) {
			f.replaceAsset("adk-key-rotation-v1.json", []byte(`{"fixture":"tampered"}`))
		}, want: "rotation document differs from bridge manifest digest"},
		{name: "rotation_signature_is_not_raw_ed25519", mutate: func(f *currentReleaseFixture) {
			f.replaceAsset("adk-key-rotation-v1.sig", bytes.Repeat([]byte{0x34}, 63))
		}, want: "rotation signature is not raw Ed25519 bytes"},
		{name: "standalone_lineage_differs_from_archive", mutate: func(f *currentReleaseFixture) {
			f.replaceAsset("release-lineage-v1.json", []byte("different standalone lineage\n"))
		}, want: "standalone and archived lineage bytes differ"},
		{name: "lineage_signature_is_not_raw_ed25519", mutate: func(f *currentReleaseFixture) {
			f.replaceAsset("release-lineage-v1.sig", bytes.Repeat([]byte{0x31}, 63))
		}, want: "lineage signature is not raw Ed25519 bytes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCurrentReleaseFixture(t)
			test.mutate(fixture)
			fixture.writeRelease(t)
			output, err := fixture.run()
			if err == nil || !strings.Contains(output, test.want) {
				t.Fatalf("untrusted evidence result: %v\n%s", err, output)
			}
			if _, statErr := os.Lstat(fixture.output); !os.IsNotExist(statErr) {
				t.Fatalf("failed verification materialized output: %v", statErr)
			}
		})
	}
}

func assertCurrentReleaseVerifierLog(t *testing.T, path string) {
	t.Helper()
	log, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(string(log)), "\n"); len(lines) != 3 {
		t.Fatalf("release verifier invocation count = %d, want 3: %s", len(lines), log)
	}
	for _, required := range []string{
		"companion-manifest-verifier --artifact ", "--platform darwin --architecture arm64",
		"omp-context-lineage-verifier --lineage ", "--target darwin-arm64 --version 0.50.109",
		"adk-channel-receiver historical",
	} {
		if !bytes.Contains(log, []byte(required)) {
			t.Fatalf("release verifier invocation missing %q: %s", required, log)
		}
	}
}

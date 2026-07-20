package companionmanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var currentReleaseArchives = []string{
	"autopus-adk_0.50.80_darwin_amd64.tar.gz",
	"autopus-adk_0.50.80_darwin_arm64.tar.gz",
	"autopus-adk_0.50.80_linux_amd64.tar.gz",
	"autopus-adk_0.50.80_linux_arm64.tar.gz",
	"autopus-adk_0.50.80_windows_amd64.tar.gz",
	"autopus-adk_0.50.80_windows_amd64.zip",
	"autopus-adk_0.50.80_windows_arm64.tar.gz",
	"autopus-adk_0.50.80_windows_arm64.zip",
}

type currentReleaseAsset struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Size   int    `json:"size"`
	Digest string `json:"digest"`
}

type currentReleaseDocument struct {
	TagName         string                `json:"tag_name"`
	TargetCommitish string                `json:"target_commitish"`
	Draft           bool                  `json:"draft"`
	Prerelease      bool                  `json:"prerelease"`
	Immutable       bool                  `json:"immutable"`
	Assets          []currentReleaseAsset `json:"assets"`
}

type currentReleaseFixture struct {
	root      string
	state     string
	output    string
	checksums []byte
	release   currentReleaseDocument
}

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
}

func TestCurrentReleaseVerifier_RejectsUntrustedReleaseEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*currentReleaseFixture)
		want   string
	}{
		{name: "mutable_release", mutate: func(f *currentReleaseFixture) {
			f.release.Immutable = false
		}, want: "not exact, final, immutable, complete, and digest-bound"},
		{name: "partial_asset_set", mutate: func(f *currentReleaseFixture) {
			f.release.Assets = f.release.Assets[:len(f.release.Assets)-1]
		}, want: "not exact, final, immutable, complete, and digest-bound"},
		{name: "duplicate_asset", mutate: func(f *currentReleaseFixture) {
			f.release.Assets[len(f.release.Assets)-1].Name = f.release.Assets[0].Name
		}, want: "not exact, final, immutable, complete, and digest-bound"},
		{name: "checksums_server_digest_mismatch", mutate: func(f *currentReleaseFixture) {
			f.asset("checksums.txt").Digest = "sha256:" + strings.Repeat("f", 64)
		}, want: "differs from its GitHub API digest"},
		{name: "cask_archive_checksum_mismatch", mutate: func(f *currentReleaseFixture) {
			f.asset(currentReleaseArchives[0]).Digest = "sha256:" + strings.Repeat("e", 64)
		}, want: "checksums.txt differs from the API digest"},
		{name: "windows_archive_checksum_mismatch", mutate: func(f *currentReleaseFixture) {
			f.asset(currentReleaseArchives[7]).Digest = "sha256:" + strings.Repeat("d", 64)
		}, want: "checksums.txt differs from the API digest"},
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

func newCurrentReleaseFixture(t *testing.T) *currentReleaseFixture {
	t.Helper()
	root := t.TempDir()
	state := filepath.Join(root, "state")
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(filepath.Join(state, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}

	var checksums bytes.Buffer
	assets := make([]currentReleaseAsset, 0, 11)
	for index, name := range currentReleaseArchives {
		digest := fmt.Sprintf("%064x", index+1)
		fmt.Fprintf(&checksums, "%s  %s\n", digest, name)
		assets = append(assets, currentReleaseAsset{
			ID: index + 1, Name: name, State: "uploaded", Size: index + 1,
			Digest: "sha256:" + digest,
		})
	}
	checksumsBytes := checksums.Bytes()
	checksumsSum := sha256.Sum256(checksumsBytes)
	assets = append(assets,
		currentReleaseAsset{ID: 9, Name: "checksums.txt", State: "uploaded",
			Size: len(checksumsBytes), Digest: fmt.Sprintf("sha256:%x", checksumsSum)},
		currentReleaseAsset{ID: 10, Name: "checksums.txt.bundle", State: "uploaded",
			Size: 1, Digest: "sha256:" + strings.Repeat("a", 64)},
		currentReleaseAsset{ID: 11, Name: "checksums.txt.signatures", State: "uploaded",
			Size: 1, Digest: "sha256:" + strings.Repeat("b", 64)},
	)
	fixture := &currentReleaseFixture{
		root: root, state: state, output: filepath.Join(root, "verified-checksums.txt"),
		checksums: append([]byte(nil), checksumsBytes...),
		release: currentReleaseDocument{
			TagName: "v0.50.80", TargetCommitish: strings.Repeat("c", 40),
			Immutable: true, Assets: assets,
		},
	}
	fixture.writeRelease(t)
	if err := os.WriteFile(filepath.Join(state, "assets", "9"), checksumsBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	mock := `#!/usr/bin/env bash
set -euo pipefail
[[ "${1-}" == api ]]
shift
endpoint=''
while (($#)); do
  case "$1" in
    -H) shift 2 ;;
    *) endpoint=$1; shift ;;
  esac
done
case "$endpoint" in
  repos/Insajin/autopus-adk/releases/tags/v0.50.80)
    exec cat "$MOCK_CURRENT_RELEASE_STATE/release.json" ;;
  repos/Insajin/autopus-adk/releases/assets/*)
    exec cat "$MOCK_CURRENT_RELEASE_STATE/assets/${endpoint##*/}" ;;
  *) exit 64 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(mock), 0o700); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f *currentReleaseFixture) asset(name string) *currentReleaseAsset {
	for index := range f.release.Assets {
		if f.release.Assets[index].Name == name {
			return &f.release.Assets[index]
		}
	}
	panic("missing release asset: " + name)
}

func (f *currentReleaseFixture) writeRelease(t *testing.T) {
	t.Helper()
	data, err := json.Marshal(f.release)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.state, "release.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *currentReleaseFixture) run() (string, error) {
	script := filepath.Join(repositoryRootForBridge(),
		"scripts/companion-release/verify-current-release.sh")
	command := exec.Command("bash", script, f.output)
	command.Env = []string{
		"PATH=" + filepath.Join(f.root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + f.root, "TMPDIR=" + f.root, "GITHUB_TOKEN=fixture-token",
		"COMPANION_SOURCE_COMMIT=" + strings.Repeat("c", 40),
		"MOCK_CURRENT_RELEASE_STATE=" + f.state,
	}
	output, err := command.CombinedOutput()
	return string(output), err
}

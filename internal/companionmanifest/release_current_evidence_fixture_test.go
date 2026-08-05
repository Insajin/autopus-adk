package companionmanifest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
	"autopus-adk_0.50.96_darwin_amd64.tar.gz",
	"autopus-adk_0.50.96_darwin_arm64.tar.gz",
	"autopus-adk_0.50.96_linux_amd64.tar.gz",
	"autopus-adk_0.50.96_linux_arm64.tar.gz",
	"autopus-adk_0.50.96_windows_amd64.tar.gz",
	"autopus-adk_0.50.96_windows_amd64.zip",
	"autopus-adk_0.50.96_windows_arm64.tar.gz",
	"autopus-adk_0.50.96_windows_arm64.zip",
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
	root              string
	state             string
	output            string
	signatureLog      string
	verifierLog       string
	openSSLVerifyFail bool
	cosignFail        bool
	checksums         []byte
	release           currentReleaseDocument
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

	autoBody := []byte("fixture distributed auto binary\n")
	autoDigest := sha256.Sum256(autoBody)
	lineage := []byte("{\"schema\":\"autopus.omp_context_release_lineage.v1\"}\n")
	lineageSignature := bytes.Repeat([]byte{0x31}, 64)
	archive := makeCurrentReleaseArchive(t, map[string][]byte{
		"auto": autoBody,
		"adk-companion-manifest.json": []byte(fmt.Sprintf(
			"{\"artifact_digest\":\"sha256:%x\"}\n", autoDigest)),
		"adk-companion-manifest.sig":                                      bytes.Repeat([]byte{0x32}, 64),
		"adk-companion-public-key-receipt.bundle/public-key-receipt.json": []byte("{\"schema\":\"autopus.adk_companion_public_key_receipt.v1\"}\n"),
		"adk-companion-public-key-receipt.bundle/public-key-receipt.sig":  bytes.Repeat([]byte{0x33}, 64),
		"release-lineage-v1.json":                                         lineage,
		"release-lineage-v1.sig":                                          lineageSignature,
	})
	assetBodies := make(map[string][]byte, 15)
	var checksums bytes.Buffer
	for _, name := range currentReleaseArchives {
		body := []byte("fixture archive " + name + "\n")
		if name == currentReleaseArchives[1] {
			body = archive
		}
		assetBodies[name] = body
		digest := sha256.Sum256(body)
		fmt.Fprintf(&checksums, "%x  %s\n", digest, name)
	}
	assetBodies["checksums.txt"] = append([]byte(nil), checksums.Bytes()...)
	assetBodies["checksums.txt.bundle"] = []byte("fixture cosign bundle\n")
	assetBodies["checksums.txt.signatures"] = []byte("AUTOPUS-RELEASE-SIGNATURE-V1\n" +
		"e1fdfe066484c7eae8ff16fa4b1ee6237b8d06299c2b66ced485f029af77837f\tMAYCAQECAQE=\n")
	assetBodies["omp-context-promotion-report.v1.json"] = []byte(
		`{"candidate":{"artifact_sha256":"` + strings.Repeat("a", 64) + `"}}`)
	assetBodies["omp-context-promotion-attestation.v2.json"] = []byte("fixture attestation\n")
	assetBodies["release-lineage-v1.json"] = lineage
	assetBodies["release-lineage-v1.sig"] = lineageSignature

	assetNames := append(append([]string(nil), currentReleaseArchives...),
		"checksums.txt", "checksums.txt.bundle", "checksums.txt.signatures",
		"omp-context-promotion-report.v1.json", "omp-context-promotion-attestation.v2.json",
		"release-lineage-v1.json", "release-lineage-v1.sig")
	assets := make([]currentReleaseAsset, 0, len(assetNames))
	for index, name := range assetNames {
		body := assetBodies[name]
		digest := sha256.Sum256(body)
		assets = append(assets, currentReleaseAsset{ID: index + 1, Name: name,
			State: "uploaded", Size: len(body), Digest: fmt.Sprintf("sha256:%x", digest)})
		if err := os.WriteFile(filepath.Join(state, "assets", fmt.Sprint(index+1)), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixture := &currentReleaseFixture{
		root: root, state: state, output: filepath.Join(root, "verified-checksums.txt"),
		signatureLog: filepath.Join(state, "signature.log"),
		verifierLog:  filepath.Join(state, "verifier.log"),
		checksums:    assetBodies["checksums.txt"],
		release: currentReleaseDocument{TagName: "v0.50.96",
			TargetCommitish: strings.Repeat("c", 40), Immutable: true, Assets: assets},
	}
	fixture.writeRelease(t)
	fixture.writeCommandMocks(t)
	return fixture
}

func makeCurrentReleaseArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	order := []string{"auto", "adk-companion-manifest.json", "adk-companion-manifest.sig",
		"adk-companion-public-key-receipt.bundle/public-key-receipt.json",
		"adk-companion-public-key-receipt.bundle/public-key-receipt.sig",
		"release-lineage-v1.json", "release-lineage-v1.sig"}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, name := range order {
		body := entries[name]
		header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func (f *currentReleaseFixture) asset(name string) *currentReleaseAsset {
	for index := range f.release.Assets {
		if f.release.Assets[index].Name == name {
			return &f.release.Assets[index]
		}
	}
	panic("missing release asset: " + name)
}

func (f *currentReleaseFixture) replaceAsset(name string, body []byte) {
	asset := f.asset(name)
	digest := sha256.Sum256(body)
	asset.Size = len(body)
	asset.Digest = fmt.Sprintf("sha256:%x", digest)
	if err := os.WriteFile(filepath.Join(f.state, "assets", fmt.Sprint(asset.ID)), body, 0o600); err != nil {
		panic(err)
	}
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
	script := filepath.Join(repositoryRootForBridge(), "scripts/companion-release/verify-current-release.sh")
	command := exec.Command("bash", script, f.output)
	if err := f.writeSignatureMocks(); err != nil {
		return "", err
	}
	command.Env = []string{
		"PATH=" + filepath.Join(f.root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + f.root, "TMPDIR=" + f.root,
		"GITHUB_TOKEN=fixture-token", "GH_TOKEN=fixture-fallback-token",
		"COMPANION_SOURCE_COMMIT=" + strings.Repeat("c", 40),
		"COMPANION_SOURCE_TREE=" + strings.Repeat("d", 40),
		"OMP_CONTEXT_EVIDENCE_REPORT_SHA256=" + f.assetDigest("omp-context-promotion-report.v1.json"),
		"OMP_CONTEXT_EVIDENCE_ATTESTATION_SHA256=" + f.assetDigest("omp-context-promotion-attestation.v2.json"),
		"OMP_CONTEXT_STATIC_POLICY_B64=eyJzY2hlbWEiOiJmaXh0dXJlIn0",
		"OMP_CONTEXT_EVIDENCE_VERIFIER=" + filepath.Join(f.root, "bin", "omp-context-verifier"),
		"OMP_CONTEXT_LINEAGE_VERIFIER=" + filepath.Join(f.root, "bin", "omp-context-lineage-verifier"),
		"COMPANION_MANIFEST_VERIFIER=" + filepath.Join(f.root, "bin", "companion-manifest-verifier"),
		"COMPANION_KEY_ID=fixture-key-v1", "COMPANION_HANDOFF=v1", "COMPANION_ROLLBACK_FLOOR=1",
		"COMPANION_PUBLIC_KEY_SHA256=sha256:" + strings.Repeat("e", 64),
		"MOCK_CURRENT_RELEASE_STATE=" + f.state,
	}
	output, err := command.CombinedOutput()
	return string(output), err
}

func (f *currentReleaseFixture) assetDigest(name string) string {
	return strings.TrimPrefix(f.asset(name).Digest, "sha256:")
}

func (f *currentReleaseFixture) writeCommandMocks(t *testing.T) {
	t.Helper()
	bin := filepath.Join(f.root, "bin")
	ghMock := `#!/usr/bin/env bash
set -euo pipefail
[[ "${1-}" == api ]]; shift
endpoint=''
while (($#)); do case "$1" in -H) shift 2 ;; *) endpoint=$1; shift ;; esac; done
case "$endpoint" in
  repos/Insajin/autopus-adk/releases/tags/v0.50.96) exec cat "$MOCK_CURRENT_RELEASE_STATE/release.json" ;;
  repos/Insajin/autopus-adk/releases/assets/*) exec cat "$MOCK_CURRENT_RELEASE_STATE/assets/${endpoint##*/}" ;;
  *) exit 64 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(ghMock), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"omp-context-verifier", "omp-context-lineage-verifier", "companion-manifest-verifier"} {
		body := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
[[ -z "${GITHUB_TOKEN+x}" && -z "${GH_TOKEN+x}" ]]
printf '%%s %%s\n' "${0##*/}" "$*" >> %q
`, f.verifierLog)
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *currentReleaseFixture) writeSignatureMocks() error {
	bin := filepath.Join(f.root, "bin")
	cosignMock := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
[[ -z "${GITHUB_TOKEN+x}" && -z "${GH_TOKEN+x}" ]]
printf 'cosign %%s\n' "$*" >> %q
[[ "${1-}" == verify-blob ]]
if [[ %t == true ]]; then exit 92; fi
`, f.signatureLog, f.cosignFail)
	if err := os.WriteFile(filepath.Join(bin, "cosign"), []byte(cosignMock), 0o700); err != nil {
		return err
	}
	realOpenSSL, err := exec.LookPath("openssl")
	if err != nil {
		return err
	}
	opensslMock := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
[[ -z "${GITHUB_TOKEN+x}" && -z "${GH_TOKEN+x}" ]]
if [[ "${1-}" == dgst ]]; then
  for argument in "$@"; do
    if [[ "$argument" == -verify ]]; then
      printf 'openssl-verify\n' >> %q
      if [[ %t == true ]]; then exit 92; fi
      exit 0
    fi
  done
fi
exec %q "$@"
`, f.signatureLog, f.openSSLVerifyFail, realOpenSSL)
	return os.WriteFile(filepath.Join(bin, "openssl"), []byte(opensslMock), 0o700)
}

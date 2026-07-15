package companionmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const exactGoReleaserModule = "github.com/goreleaser/goreleaser/v2@v2.17.0"

type executableLineageAsset struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Digest string `json:"digest"`
}

type executableLineageRelease struct {
	TagName         string                   `json:"tag_name"`
	TargetCommitish string                   `json:"target_commitish"`
	Draft           bool                     `json:"draft"`
	Prerelease      bool                     `json:"prerelease"`
	Immutable       bool                     `json:"immutable"`
	Assets          []executableLineageAsset `json:"assets"`
}

// The archive bytes come from GoReleaser; Apple trust events remain synthetic test inputs.
func produceGoReleaserA0FixtureEvidence(
	t *testing.T,
	tools executableLineageTools,
) *goReleaserA0Evidence {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repository")
	copyLineageRepository(t, root)
	runLineageCommand(t, root, "git", "init", "-q")
	runLineageCommand(t, root, "git", "config", "user.name", "F07 Release Test")
	runLineageCommand(t, root, "git", "config", "user.email", "f07@example.invalid")
	runLineageCommand(t, root, "git", "remote", "add", "origin", "https://example.invalid/Insajin/autopus-adk.git")
	runLineageCommand(t, root, "git", "add", ".")
	runLineageCommand(t, root, "git", "commit", "-qm", "F07 exact GoReleaser A0 fixture")
	runLineageCommand(t, root, "git", "tag", publicKeyReceiptA0Tag)
	commit := strings.TrimSpace(runLineageCommand(t, root, "git", "rev-parse", "HEAD"))

	credentialRoot := filepath.Join(filepath.Dir(root), "credentials")
	if err := os.Mkdir(credentialRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	seed := sha256.Sum256([]byte("autopus-f07-lineage-ed25519-seed"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	keyPath := filepath.Join(credentialRoot, "release-key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(privateKey)), 0o600); err != nil {
		t.Fatal(err)
	}
	tmpDir := filepath.Join(filepath.Dir(root), "tmp")
	if err := os.Mkdir(tmpDir, 0o700); err != nil {
		t.Fatal(err)
	}
	toolEnv := darwinReleaseToolEnv(t, credentialRoot)
	environment := append(os.Environ(), toolEnv...)
	environment = append(environment,
		"GO_WANT_COMPANION_SIGNER_HELPER=1", "TMPDIR="+tmpDir,
		"GITHUB_REF_NAME="+publicKeyReceiptA0Tag, "COMPANION_SOURCE_COMMIT="+commit,
		"COMPANION_BUILD_PROVENANCE=github-actions:Insajin/autopus-adk@"+commit,
		"COMPANION_HANDOFF="+lineageHandoff, "COMPANION_ROLLBACK_FLOOR=5069",
		"COMPANION_ISSUED_AT=2026-07-15T00:00:00Z",
		"COMPANION_EXPIRES_AT=2026-07-16T00:00:00Z", "COMPANION_KEY_ID="+lineageKeyID,
		"COMPANION_SIGNING_KEY_FILE="+keyPath, "COMPANION_SIGNER="+tools.signer,
		"COMPANION_RECEIPT_VERIFIER="+tools.verifier,
		"COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT="+lineageIssuedAt,
		"COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT="+lineageExpiresAt,
		"COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS=31536000",
		"HOMEBREW_TAP_TOKEN=fixture-token",
	)
	args := []string{"run", exactGoReleaserModule, "release", "--clean", "--parallelism=2",
		"--skip=announce,publish,sign,homebrew"}
	command := exec.Command("go", args...)
	command.Dir, command.Env = root, environment
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("exact GoReleaser A0 output: %v\n%s", err, output)
	}

	evidence := &goReleaserA0Evidence{commit: commit, archives: map[string][]byte{}}
	for _, architecture := range []string{"amd64", "arm64"} {
		name := fmt.Sprintf("autopus-adk_%s_darwin_%s.tar.gz", lineageA0Version, architecture)
		evidence.archives[architecture] = readLineageFile(t, filepath.Join(root, "dist", name))
		entries, err := decodeLineageArchive(evidence.archives[architecture])
		if err != nil {
			t.Fatalf("decode exact %s archive: %v", architecture, err)
		}
		manifestBytes := entries["adk-companion-manifest.json"].data
		if architecture == "amd64" {
			evidence.receipt = entries[lineageBundleName+"/public-key-receipt.json"].data
			evidence.signature = entries[lineageBundleName+"/public-key-receipt.sig"].data
			evidence.pins.amd64Manifest = lineageDigest(manifestBytes)
		} else {
			evidence.pins.arm64Manifest = lineageDigest(manifestBytes)
		}
	}
	evidence.checksums = readLineageFile(t, filepath.Join(root, "dist", "checksums.txt"))
	evidence.pins = captureGoReleaserPins(evidence, privateKey)
	return evidence
}

func captureGoReleaserPins(
	evidence *goReleaserA0Evidence,
	privateKey ed25519.PrivateKey,
) executableLineagePins {
	amd64, _ := decodeLineageArchive(evidence.archives["amd64"])
	arm64, _ := decodeLineageArchive(evidence.archives["arm64"])
	receiptDigest, signatureDigest := lineageDigest(evidence.receipt), lineageDigest(evidence.signature)
	return executableLineagePins{
		commit: evidence.commit, receipt: receiptDigest, signature: signatureDigest,
		record:        lineageRecordDigest(receiptDigest, signatureDigest),
		publicKey:     lineageDigest(privateKey.Public().(ed25519.PublicKey)),
		checksums:     lineageDigest(evidence.checksums),
		amd64Manifest: lineageDigest(amd64["adk-companion-manifest.json"].data),
		arm64Manifest: lineageDigest(arm64["adk-companion-manifest.json"].data),
	}
}

func (fixture *executableLineageFixture) writeEvidence(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll(fixture.assetsDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.assetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	assets := make([]executableLineageAsset, 0, 3)
	for _, architecture := range []string{"amd64", "arm64"} {
		name := fmt.Sprintf("autopus-adk_%s_darwin_%s.tar.gz", lineageA0Version, architecture)
		data := append([]byte(nil), fixture.evidence.archives[architecture]...)
		if fixture.archiveMutation != nil {
			data = fixture.archiveMutation(t, architecture, data)
		}
		if fixture.omitSignatureEntry {
			data = rewriteLineageArchive(t, data, func(name string, entry []byte) ([]byte, bool) {
				return entry, name != lineageBundleName+"/public-key-receipt.sig"
			})
		}
		if err := os.WriteFile(filepath.Join(fixture.assetsDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := "sha256:" + lineageDigest(data)
		if architecture == "amd64" && fixture.assetDigestOverride != "" {
			digest = fixture.assetDigestOverride
		}
		assets = append(assets, executableLineageAsset{Name: name, State: "uploaded", Digest: digest})
	}
	checksumsName := "checksums.txt"
	if err := os.WriteFile(filepath.Join(fixture.assetsDir, checksumsName), fixture.checksums, 0o600); err != nil {
		t.Fatal(err)
	}
	assets = append(assets, executableLineageAsset{
		Name: checksumsName, State: "uploaded", Digest: "sha256:" + lineageDigest(fixture.checksums),
	})
	writeLineageJSON(t, fixture.releaseJSON, executableLineageRelease{
		TagName: fixture.releaseTag, TargetCommitish: fixture.targetCommit, Immutable: true, Assets: assets,
	})
	writeLineageJSON(t, fixture.tagJSON, map[string]any{
		"object": map[string]string{"type": "commit", "sha": fixture.tagCommit},
	})
	writeLineageJSON(t, fixture.commitJSON, map[string]string{"sha": fixture.evidence.commit})
}

func (fixture *executableLineageFixture) writeMockGitHub(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(fixture.mockToolsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fixture.mockToolsDir, "gh")
	source := `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1-}" == 'api' ]]; then
  case "${2-}" in
    */releases/tags/*) exec /bin/cat ` + shellQuote(fixture.releaseJSON) + ` ;;
    */git/ref/tags/*) exec /bin/cat ` + shellQuote(fixture.tagJSON) + ` ;;
    */commits/*) exec /bin/cat ` + shellQuote(fixture.commitJSON) + ` ;;
    *) exit 64 ;;
  esac
fi
if [[ "${1-}" == 'release' && "${2-}" == 'download' ]]; then
  pattern=''; destination=''; shift 2
  while (($#)); do
    case "$1" in
      --pattern) pattern="$2"; shift 2 ;;
      --dir) destination="$2"; shift 2 ;;
      *) shift ;;
    esac
  done
  [[ -n "$pattern" && -n "$destination" ]] || exit 64
  exec /bin/cp ` + shellQuote(fixture.assetsDir) + `/"$pattern" "$destination/$pattern"
fi
exit 64
`
	if err := os.WriteFile(path, []byte(source), 0o700); err != nil {
		t.Fatal(err)
	}
}

func (fixture *executableLineageFixture) writeProvisionedProductionScript(t *testing.T) {
	t.Helper()
	source := string(releaseSourceFile(t, "scripts/companion-release/verify-public-key-lineage.sh"))
	replacements := map[string]string{
		"A0_COMMIT_SHA": fixture.pins.commit, "A0_RECEIPT_SHA256": fixture.pins.receipt,
		"A0_SIGNATURE_SHA256": fixture.pins.signature, "A0_RECORD_SHA256": fixture.pins.record,
		"A0_PUBLIC_KEY_SHA256": fixture.pins.publicKey, "A0_CHECKSUMS_SHA256": fixture.pins.checksums,
		"A0_AMD64_MANIFEST_SHA256": fixture.pins.amd64Manifest,
		"A0_ARM64_MANIFEST_SHA256": fixture.pins.arm64Manifest,
	}
	for name, value := range replacements {
		blank := "readonly " + name + "=''"
		if strings.Count(source, blank) != 1 {
			t.Fatalf("production lineage pin declaration %s is not exact", name)
		}
		source = strings.Replace(source, blank, "readonly "+name+"='"+value+"'", 1)
	}
	if err := os.WriteFile(fixture.provisionedScriptPath, []byte(source), 0o700); err != nil {
		t.Fatal(err)
	}
}

func writeLineageJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

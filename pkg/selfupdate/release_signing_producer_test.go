package selfupdate

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReleaseSigningProducer_DualSignaturesAreSortedAndVerifiable(t *testing.T) {
	requireReleaseSigningShell(t)
	requireTool(t, "openssl")
	root := releaseSigningRepositoryRoot(t)
	producer := filepath.Join(root, "scripts", "release-signing", "sign-checksums.sh")
	dir := t.TempDir()
	checksums := []byte("abc123  autopus-adk_0.50.73_darwin_arm64.tar.gz\n")
	checksumsPath := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumsPath, checksums, 0o600))

	private1, pinned1 := generateReleaseTestKey(t, "2099-12-31")
	private2, pinned2 := generateReleaseTestKey(t, "2099-12-31")
	key1 := writeReleasePrivateKey(t, dir, "k1.pem", private1)
	key2 := writeReleasePrivateKey(t, dir, "k2.pem", private2)
	output := filepath.Join(dir, "checksums.txt.signatures")

	command := exec.Command(producer, checksumsPath, output, key2, key1)
	combined, err := command.CombinedOutput()
	require.NoError(t, err, "producer failed: %s", combined)
	envelope, err := os.ReadFile(output)
	require.NoError(t, err)
	require.NoError(t, verifyReleaseSignatures(
		checksums, envelope, []pinnedReleaseKey{pinned1, pinned2}, referenceTime,
	))

	entries, err := parseReleaseSignatureEnvelope(envelope)
	require.NoError(t, err)
	fingerprints := []string{entries[0].fingerprint, entries[1].fingerprint}
	want := append([]string(nil), fingerprints...)
	sort.Strings(want)
	require.Equal(t, want, fingerprints)
}

func TestReleaseSigningProducer_RejectsDuplicateAndNonP256Keys(t *testing.T) {
	requireReleaseSigningShell(t)
	requireTool(t, "openssl")
	root := releaseSigningRepositoryRoot(t)
	producer := filepath.Join(root, "scripts", "release-signing", "sign-checksums.sh")
	dir := t.TempDir()
	checksumsPath := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumsPath, []byte("fixture\n"), 0o600))

	private, _ := generateReleaseTestKey(t, "2099-12-31")
	key := writeReleasePrivateKey(t, dir, "key.pem", private)
	output := filepath.Join(dir, "checksums.txt.signatures")
	combined, err := exec.Command(producer, checksumsPath, output, key, key).CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(combined), "duplicate")
	_, statErr := os.Stat(output)
	require.ErrorIs(t, statErr, os.ErrNotExist, "duplicate-key failure must not publish a completed envelope")

	p384Command := exec.Command("openssl", "ecparam", "-name", "secp384r1", "-genkey", "-noout", "-out", filepath.Join(dir, "p384.pem"))
	require.NoError(t, p384Command.Run())
	combined, err = exec.Command(producer, checksumsPath, output, filepath.Join(dir, "p384.pem")).CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(combined), "P-256")
	_, statErr = os.Stat(output)
	require.ErrorIs(t, statErr, os.ErrNotExist, "non-P-256 failure must not publish a completed envelope")
}

func TestReleaseSigningPreflight_RequiresExactPrivatePublicFingerprintTuple(t *testing.T) {
	requireReleaseSigningShell(t)
	requireTool(t, "openssl")
	root := releaseSigningRepositoryRoot(t)
	preflight := filepath.Join(root, "scripts", "release-signing", "verify-key-pair.sh")
	dir := t.TempDir()
	private, pinned := generateReleaseTestKey(t, "2099-12-31")
	privatePath := writeReleasePrivateKey(t, dir, "private.pem", private)
	publicPath := filepath.Join(dir, "public.pem")
	fingerprintPath := filepath.Join(dir, "fingerprint")
	require.NoError(t, os.WriteFile(publicPath, []byte(pinned.PublicKeyPEM), 0o600))
	require.NoError(t, os.WriteFile(fingerprintPath, []byte(pinned.Fingerprint+"\n"), 0o600))

	combined, err := exec.Command(preflight, privatePath, publicPath, fingerprintPath).CombinedOutput()
	require.NoError(t, err, "preflight failed: %s", combined)

	otherPrivate, _ := generateReleaseTestKey(t, "2099-12-31")
	otherPrivatePath := writeReleasePrivateKey(t, dir, "other.pem", otherPrivate)
	combined, err = exec.Command(preflight, otherPrivatePath, publicPath, fingerprintPath).CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(combined), "does not pair")
}

func TestReleaseSigningProducer_GoReleaserV217ExecutableOracle(t *testing.T) {
	requireReleaseSigningShell(t)
	requireTool(t, "openssl")
	goReleaser := os.Getenv("GORELEASER_V217_BIN")
	if goReleaser == "" {
		t.Skip("set GORELEASER_V217_BIN to run the pinned GoReleaser v2.17.0 oracle")
	}
	versionOutput, err := exec.Command(goReleaser, "--version").CombinedOutput()
	require.NoError(t, err, "goreleaser --version: %s", versionOutput)
	require.Contains(t, string(versionOutput), "2.17.0")

	root := releaseSigningRepositoryRoot(t)
	temp := t.TempDir()
	repo := filepath.Join(temp, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "scripts", "release-signing"), 0o700))
	copyTestFile(t,
		filepath.Join(root, "scripts", "release-signing", "sign-checksums.sh"),
		filepath.Join(repo, "scripts", "release-signing", "sign-checksums.sh"), 0o700,
	)
	writeOracleProject(t, repo)

	private, pinned := generateReleaseTestKey(t, "2099-12-31")
	keyPath := writeReleasePrivateKey(t, temp, "release-key.pem", private)
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "oracle@example.test")
	runGit(t, repo, "config", "user.name", "Release Oracle")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "oracle fixture")

	command := exec.Command(goReleaser, "release", "--snapshot", "--clean")
	command.Dir = repo
	command.Env = append(os.Environ(), "ADK_RELEASE_ECDSA_PRIVATE_KEY_FILE="+keyPath)
	combined, err := command.CombinedOutput()
	require.NoError(t, err, "GoReleaser oracle failed: %s", combined)

	checksums, err := os.ReadFile(filepath.Join(repo, "dist", "checksums.txt"))
	require.NoError(t, err)
	envelope, err := os.ReadFile(filepath.Join(repo, "dist", "checksums.txt.signatures"))
	require.NoError(t, err)
	require.NoError(t, verifyReleaseSignatures(checksums, envelope, []pinnedReleaseKey{pinned}, referenceTime))
}

func writeOracleProject(t *testing.T, repo string) {
	t.Helper()
	mainSource := "package main\nfunc main() {}\n"
	config := `version: 2
project_name: oracle
builds:
  - id: oracle
    main: .
    binary: oracle
    goos: [darwin]
    goarch: [arm64]
checksum:
  name_template: checksums.txt
signs:
  - id: ecdsa-release-envelope
    cmd: scripts/release-signing/sign-checksums.sh
    artifacts: checksum
    args:
      - "${artifact}"
      - "${signature}"
      - "{{ .Env.ADK_RELEASE_ECDSA_PRIVATE_KEY_FILE }}"
    signature: "${artifact}.signatures"
    output: true
release:
  disable: true
`
	require.NoError(t, os.WriteFile(filepath.Join(repo, "main.go"), []byte(mainSource), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.test/oracle\n\ngo 1.26\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".goreleaser.yaml"), []byte(config), 0o600))
}

func writeReleasePrivateKey(t *testing.T, dir, name string, private *ecdsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(private)
	require.NoError(t, err)
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600))
	return path
}

func copyTestFile(t *testing.T, source, destination string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(source)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(destination, data, mode))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), output)
}

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is required for release-signing oracle", name)
	}
}

func requireReleaseSigningShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("release-signing shell oracles require a POSIX host")
	}
}

func releaseSigningRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

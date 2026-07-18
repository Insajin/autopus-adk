package companionmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const (
	lineageKeyID      = "release-key"
	lineageHandoff    = "v1"
	lineageFloor      = uint64(5069)
	lineageIssuedAt   = "2026-07-14T00:00:00Z"
	lineageExpiresAt  = "2027-07-15T00:00:00Z"
	lineageA0Version  = "0.50.69"
	lineageA1Version  = "0.50.70"
	lineageBundleName = "adk-companion-public-key-receipt.bundle"
)

type executableLineageTools struct {
	signer   string
	verifier string
}

type executableLineagePins struct {
	commit        string
	tagObject     string
	receipt       string
	signature     string
	record        string
	publicKey     string
	checksums     string
	amd64Archive  string
	arm64Archive  string
	amd64Manifest string
	arm64Manifest string
}

type goReleaserA0Evidence struct {
	tag       string
	version   string
	commit    string
	archives  map[string][]byte
	checksums []byte
	receipt   []byte
	signature []byte
	pins      executableLineagePins
}

type lineageArchiveMutation func(*testing.T, string, []byte) []byte

type executableLineageFixture struct {
	root                  string
	tools                 executableLineageTools
	privateKey            ed25519.PrivateKey
	keyPath               string
	evidence              *goReleaserA0Evidence
	pins                  executableLineagePins
	issuedAt              string
	expiresAt             string
	targetCommit          string
	tagCommit             string
	tagObject             string
	releaseTag            string
	currentTag            string
	token                 string
	checksums             []byte
	archiveMutation       lineageArchiveMutation
	assetDigestOverride   string
	omitSignatureEntry    bool
	releaseJSON           string
	tagJSON               string
	annotatedTagJSON      string
	commitJSON            string
	assetsDir             string
	mockToolsDir          string
	provisionedScriptPath string
}

var (
	sharedExecutableLineageSigner   []byte
	sharedExecutableLineageVerifier []byte
	executableLineageToolsOnce      sync.Once
	executableLineageToolsBuildRuns atomic.Int32
)

func newExecutableLineageTools(t *testing.T) executableLineageTools {
	t.Helper()
	requireExecutableLineageIntegration(t)
	executableLineageToolsOnce.Do(func() {
		root := t.TempDir()
		tools := executableLineageTools{
			signer:   filepath.Join(root, "auto-companion-manifest-signer"),
			verifier: filepath.Join(root, "auto-companion-receipt-verifier"),
		}
		buildExecutableLineageBinary(t, tools.signer, "./cmd/auto")
		buildExecutableLineageBinary(t, tools.verifier,
			"./internal/companionmanifest/receiptverify")
		sharedExecutableLineageSigner = readLineageFile(t, tools.signer)
		sharedExecutableLineageVerifier = readLineageFile(t, tools.verifier)
		executableLineageToolsBuildRuns.Add(1)
	})
	root := t.TempDir()
	tools := executableLineageTools{
		signer:   filepath.Join(root, "auto-companion-manifest-signer"),
		verifier: filepath.Join(root, "auto-companion-receipt-verifier"),
	}
	if err := os.WriteFile(tools.signer, sharedExecutableLineageSigner, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tools.verifier, sharedExecutableLineageVerifier, 0o700); err != nil {
		t.Fatal(err)
	}
	return tools
}

func requireExecutableLineageIntegration(t *testing.T) {
	t.Helper()
	if !executableLineageIntegrationEnabled {
		t.Skip("executable GoReleaser lineage fixture requires -tags integration")
	}
}

func newExecutableLineageFixture(
	t *testing.T,
	tools executableLineageTools,
	evidence *goReleaserA0Evidence,
) *executableLineageFixture {
	t.Helper()
	root := t.TempDir()
	seed := sha256.Sum256([]byte("autopus-f07-lineage-ed25519-seed"))
	currentTag := publicKeyReceiptA1Tag
	switch evidence.version {
	case lineageA1Version:
		currentTag = publicKeyReceiptA2Tag
	case publicKeyReceiptA2Version:
		currentTag = publicKeyReceiptA3Tag
	case publicKeyReceiptA3Version:
		currentTag = publicKeyReceiptA4Tag
	case publicKeyReceiptA4Version:
		currentTag = publicKeyReceiptA5Tag
	}
	fixture := &executableLineageFixture{
		root: root, tools: tools, evidence: evidence, pins: evidence.pins,
		keyPath: filepath.Join(root, "release-key"), issuedAt: lineageIssuedAt,
		expiresAt: lineageExpiresAt, targetCommit: evidence.commit, tagCommit: evidence.commit,
		tagObject:  evidence.pins.tagObject,
		releaseTag: evidence.tag, currentTag: currentTag, token: "f07-mock-github-token",
		checksums: append([]byte(nil), evidence.checksums...),
		assetsDir: filepath.Join(root, "assets"), mockToolsDir: filepath.Join(root, "tools"),
		releaseJSON: filepath.Join(root, "release.json"), tagJSON: filepath.Join(root, "tag.json"),
		annotatedTagJSON:      filepath.Join(root, "annotated-tag.json"),
		commitJSON:            filepath.Join(root, "commit.json"),
		provisionedScriptPath: filepath.Join(root, "verify-public-key-lineage.sh"),
	}
	fixture.writeSigningKey(t, seed[:])
	fixture.writeEvidence(t)
	return fixture
}

func (fixture *executableLineageFixture) writeSigningKey(t *testing.T, seed []byte) {
	t.Helper()
	privateKey := ed25519.NewKeyFromSeed(seed)
	fixture.privateKey = privateKey
	encoded := base64.StdEncoding.EncodeToString(privateKey)
	if err := os.WriteFile(fixture.keyPath, []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (fixture *executableLineageFixture) run(t *testing.T) (string, error) {
	t.Helper()
	fixture.writeMockGitHub(t)
	fixture.writeProvisionedProductionScript(t)
	home, tmp := filepath.Join(fixture.root, "home"), filepath.Join(fixture.root, "tmp")
	for _, dir := range []string{home, tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("bash", fixture.provisionedScriptPath)
	command.Env = []string{
		"PATH=" + fixture.mockToolsDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + home, "TMPDIR=" + tmp, "GITHUB_REF_NAME=" + fixture.currentTag,
		"GITHUB_TOKEN=" + fixture.token, "COMPANION_SIGNER=" + fixture.tools.signer,
		"COMPANION_RECEIPT_VERIFIER=" + fixture.tools.verifier,
		"COMPANION_SIGNING_KEY_FILE=" + fixture.keyPath, "COMPANION_KEY_ID=" + lineageKeyID,
		"COMPANION_HANDOFF=" + lineageHandoff,
		"COMPANION_ROLLBACK_FLOOR=" + strconv.FormatUint(lineageFloor, 10),
		"COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT=" + fixture.issuedAt,
		"COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT=" + fixture.expiresAt,
	}
	output, err := command.CombinedOutput()
	return string(output), err
}

func buildExecutableLineageBinary(t *testing.T, output, packagePath string) {
	t.Helper()
	command := exec.Command("go", "build", "-trimpath", "-o", output, packagePath)
	command.Dir = lineageRepositoryRoot(t)
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", packagePath, err, result)
	}
	if err := os.Chmod(output, 0o700); err != nil {
		t.Fatal(err)
	}
}

func lineageDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func lineageRecordDigest(receiptDigest, signatureDigest string) string {
	receipt := mustDecodeLineageDigest(receiptDigest)
	signature := mustDecodeLineageDigest(signatureDigest)
	record := append([]byte("autopus.public-key-receipt.a0-record.v1\x00"), receipt...)
	return lineageDigest(append(record, signature...))
}

func mustDecodeLineageDigest(value string) []byte {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		panic("invalid test lineage digest")
	}
	return decoded
}

func readLineageFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

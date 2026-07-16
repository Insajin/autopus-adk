package cli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

const maxEncodedPrivateKeyBytes = 4096

type companionManifestSignOptions struct {
	artifactPath    string
	manifestOutput  string
	signatureOutput string
	version         string
	platform        string
	architecture    string
	buildProvenance string
	handoff         string
	rollbackFloor   uint64
	issuedAt        string
	expiresAt       string
	keyID           string
}

type companionManifestSignReceipt struct {
	SchemaVersion     string `json:"schema_version"`
	ArtifactDigest    string `json:"artifact_digest"`
	KeyID             string `json:"key_id"`
	ManifestSHA256    string `json:"manifest_sha256"`
	SignatureEncoding string `json:"signature_encoding"`
	Status            string `json:"status"`
}

func newCompanionManifestCmd() *cobra.Command {
	command := &cobra.Command{
		Use:          "companion-manifest",
		Short:        "Produce signed ADK companion release manifests",
		SilenceUsage: true,
	}
	command.AddCommand(newCompanionManifestSignCmd())
	command.AddCommand(newCompanionPublicKeyReceiptCmd())
	return command
}

func newCompanionManifestSignCmd() *cobra.Command {
	var options companionManifestSignOptions
	command := &cobra.Command{
		Use:          "sign",
		Short:        "Sign a canonical companion manifest using a stdin key",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return runCompanionManifestSign(command, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.artifactPath, "artifact", "", "Artifact file to digest")
	flags.StringVar(&options.manifestOutput, "manifest-output", "", "Canonical manifest output file")
	flags.StringVar(&options.signatureOutput, "signature-output", "", "Raw detached signature output file")
	flags.StringVar(&options.version, "version", "", "Artifact version")
	flags.StringVar(&options.platform, "platform", "", "Artifact platform")
	flags.StringVar(&options.architecture, "architecture", "", "Artifact architecture")
	flags.StringVar(&options.buildProvenance, "build-provenance", "", "Artifact build provenance")
	flags.StringVar(&options.handoff, "handoff", "", "Desktop handoff contract")
	flags.Uint64Var(&options.rollbackFloor, "rollback-floor", 0, "Monotonic rollback floor")
	flags.StringVar(&options.issuedAt, "issued-at", "", "Canonical UTC issuance time")
	flags.StringVar(&options.expiresAt, "expires-at", "", "Canonical UTC expiry time")
	flags.StringVar(&options.keyID, "key-id", "", "Pinned public key identifier")
	for _, name := range []string{
		"artifact", "manifest-output", "signature-output", "version", "platform", "architecture",
		"build-provenance", "handoff", "rollback-floor", "issued-at", "expires-at", "key-id",
	} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func runCompanionManifestSign(command *cobra.Command, options companionManifestSignOptions) error {
	paths, err := resolveDistinctCompanionPaths(
		options.artifactPath, options.manifestOutput, options.signatureOutput,
	)
	if err != nil {
		return err
	}
	artifactDigest, err := digestRegularFile(paths[0])
	if err != nil {
		return err
	}
	privateKey, err := readPrivateKey(command.InOrStdin())
	if err != nil {
		return err
	}
	defer clear(privateKey)
	manifest := companionmanifest.Manifest{
		SchemaVersion: companionmanifest.SchemaVersion, ArtifactDigest: artifactDigest,
		Version: options.version, Platform: options.platform, Architecture: options.architecture,
		BuildProvenance: options.buildProvenance, Handoff: options.handoff,
		RollbackFloor: options.rollbackFloor, IssuedAt: options.issuedAt,
		ExpiresAt: options.expiresAt, KeyID: options.keyID,
	}
	manifestBytes, signature, err := companionmanifest.SignCanonical(manifest, privateKey)
	if err != nil {
		return err
	}
	if err := companionmanifest.WriteSignedFiles(
		paths[1], paths[2], manifestBytes, signature,
	); err != nil {
		return err
	}
	manifestSum := sha256.Sum256(manifestBytes)
	receipt := companionManifestSignReceipt{
		SchemaVersion: "adk-companion-manifest-sign-result.v1", ArtifactDigest: artifactDigest,
		KeyID: options.keyID, ManifestSHA256: "sha256:" + hex.EncodeToString(manifestSum[:]),
		SignatureEncoding: "ed25519-raw", Status: "signed",
	}
	if err := json.NewEncoder(command.OutOrStdout()).Encode(receipt); err != nil {
		return errors.New("encode signing receipt")
	}
	return nil
}

func resolveDistinctCompanionPaths(artifact, manifest, signature string) ([3]string, error) {
	paths := [3]string{artifact, manifest, signature}
	var identities [3]os.FileInfo
	for index, path := range paths {
		absolute, err := filepath.Abs(filepath.Clean(path))
		if err != nil {
			return [3]string{}, errors.New("resolve companion path")
		}
		paths[index] = absolute
		identity, statErr := os.Stat(absolute)
		if statErr == nil {
			identities[index] = identity
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return [3]string{}, errors.New("inspect companion path")
		}
	}
	if paths[0] == paths[1] || paths[0] == paths[2] || paths[1] == paths[2] {
		return [3]string{}, errors.New("artifact and signed outputs must be distinct")
	}
	for _, pair := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		left, right := identities[pair[0]], identities[pair[1]]
		if left != nil && right != nil && os.SameFile(left, right) {
			return [3]string{}, errors.New("artifact and signed outputs must have distinct identities")
		}
	}
	return paths, nil
}

func readPrivateKey(input io.Reader) (ed25519.PrivateKey, error) {
	raw, err := io.ReadAll(io.LimitReader(input, maxEncodedPrivateKeyBytes+1))
	if err != nil || len(raw) > maxEncodedPrivateKeyBytes {
		return nil, errors.New("read signing key from stdin")
	}
	defer clear(raw)
	encoded := bytes.TrimSpace(raw)
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	decodedLength, err := base64.StdEncoding.Decode(decoded, encoded)
	if err != nil || decodedLength != ed25519.PrivateKeySize {
		clear(decoded)
		return nil, errors.New("invalid stdin signing key")
	}
	return ed25519.PrivateKey(decoded[:decodedLength]), nil
}

func digestRegularFile(path string) (string, error) {
	return digestRegularFileWithPostReadHook(path, nil)
}

func digestRegularFileWithPostReadHook(path string, postReadHook func()) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("artifact must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", errors.New("open artifact")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return "", errors.New("artifact identity changed while opening")
	}
	firstDigest, err := digestOpenArtifact(file)
	if err != nil {
		return "", errors.New("digest artifact")
	}
	if postReadHook != nil {
		postReadHook()
	}
	if !artifactPathMatchesOpenFile(path, info, file) {
		return "", errors.New("artifact identity changed while digesting")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("rewind artifact")
	}
	secondDigest, err := digestOpenArtifact(file)
	if err != nil {
		return "", errors.New("redigest artifact")
	}
	if !bytes.Equal(firstDigest, secondDigest) ||
		!artifactPathMatchesOpenFile(path, info, file) {
		return "", errors.New("artifact changed while digesting")
	}
	return "sha256:" + hex.EncodeToString(firstDigest), nil
}

func digestOpenArtifact(file *os.File) ([]byte, error) {
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return nil, err
	}
	return digest.Sum(nil), nil
}

func artifactPathMatchesOpenFile(path string, initial os.FileInfo, file *os.File) bool {
	pathInfo, pathErr := os.Lstat(path)
	openedInfo, openedErr := file.Stat()
	return pathErr == nil && openedErr == nil && pathInfo.Mode().IsRegular() &&
		openedInfo.Mode().IsRegular() && os.SameFile(initial, pathInfo) &&
		os.SameFile(initial, openedInfo)
}

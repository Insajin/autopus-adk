package cli

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

type companionOMPContextLineageOptions struct {
	keyID            string
	upstreamSHA256   string
	executableSHA256 string
	sourceRepository string
	sourceCommit     string
	sourceTree       string
	target           string
	version          string
	lineageOutput    string
	signatureOutput  string
}

type companionOMPContextLineageSignReceipt struct {
	SchemaVersion     string `json:"schema_version"`
	KeyID             string `json:"key_id"`
	LineageSHA256     string `json:"lineage_sha256"`
	SignatureEncoding string `json:"signature_encoding"`
	Status            string `json:"status"`
}

func newCompanionOMPContextReleaseLineageCmd() *cobra.Command {
	var options companionOMPContextLineageOptions
	command := &cobra.Command{
		Use:   "omp-context-release-lineage",
		Short: "Sign canonical OMP context release lineage using a stdin key",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return errors.New("OMP context release lineage accepts no positional arguments")
			}
			return nil
		},
		SilenceUsage: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return runCompanionOMPContextReleaseLineage(command, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.keyID, "key-id", "", "Pinned release key identifier")
	flags.StringVar(&options.upstreamSHA256, "upstream-sha256", "", "U candidate digest with sha256: prefix")
	flags.StringVar(&options.executableSHA256, "executable-sha256", "", "D executable digest with sha256: prefix")
	flags.StringVar(&options.sourceRepository, "source-repository", "", "Source repository coordinate")
	flags.StringVar(&options.sourceCommit, "source-commit", "", "Source commit object ID")
	flags.StringVar(&options.sourceTree, "source-tree", "", "Source tree object ID")
	flags.StringVar(&options.target, "target", "", "Release target")
	flags.StringVar(&options.version, "version", "", "Release version")
	flags.StringVar(&options.lineageOutput, "lineage-output", "", "Canonical lineage output file")
	flags.StringVar(&options.signatureOutput, "signature-output", "", "Raw detached signature output file")
	for _, name := range []string{
		"key-id", "upstream-sha256", "executable-sha256", "source-repository",
		"source-commit", "source-tree", "target", "version", "lineage-output", "signature-output",
	} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func runCompanionOMPContextReleaseLineage(
	command *cobra.Command,
	options companionOMPContextLineageOptions,
) error {
	lineagePath, signaturePath, err := resolveNewOMPContextLineageOutputs(
		options.lineageOutput,
		options.signatureOutput,
	)
	if err != nil {
		return err
	}
	lineage := companionmanifest.OMPContextReleaseLineageV1{
		SchemaVersion: companionmanifest.OMPContextReleaseLineageSchemaV1,
		KeyID:         options.keyID, Algorithm: "ed25519",
		UpstreamSHA256: options.upstreamSHA256, ExecutableSHA256: options.executableSHA256,
		SourceRepository: options.sourceRepository, SourceCommit: options.sourceCommit,
		SourceTree: options.sourceTree, Target: options.target, Version: options.version,
	}
	if _, err := companionmanifest.CanonicalOMPContextReleaseLineageBytes(lineage); err != nil {
		return err
	}
	privateKey, err := readCompanionManifestSigningKey(command.InOrStdin())
	if err != nil {
		return err
	}
	defer clear(privateKey)
	lineageBytes, signature, err := companionmanifest.SignCanonicalOMPContextReleaseLineage(
		lineage,
		privateKey,
	)
	if err != nil {
		return err
	}
	if err := companionmanifest.WriteSignedFiles(
		lineagePath,
		signaturePath,
		lineageBytes,
		signature,
	); err != nil {
		return err
	}
	digest := sha256.Sum256(lineageBytes)
	receipt := companionOMPContextLineageSignReceipt{
		SchemaVersion:     "autopus.omp_context_release_lineage_sign_result.v1",
		KeyID:             options.keyID,
		LineageSHA256:     "sha256:" + hex.EncodeToString(digest[:]),
		SignatureEncoding: "ed25519-raw",
		Status:            "signed",
	}
	if err := json.NewEncoder(command.OutOrStdout()).Encode(receipt); err != nil {
		return errors.New("encode OMP context release lineage signing receipt")
	}
	return nil
}

func readCompanionManifestSigningKey(input io.Reader) (ed25519.PrivateKey, error) {
	return readPrivateKey(input)
}

func resolveNewOMPContextLineageOutputs(
	lineagePath,
	signaturePath string,
) (string, string, error) {
	paths := []string{lineagePath, signaturePath}
	for index, path := range paths {
		absolute, err := filepath.Abs(filepath.Clean(path))
		if err != nil {
			return "", "", errors.New("resolve OMP context release lineage output")
		}
		paths[index] = absolute
	}
	if paths[0] == paths[1] || filepath.Dir(paths[0]) != filepath.Dir(paths[1]) {
		return "", "", errors.New("OMP context release lineage outputs must be distinct files in one directory")
	}
	parentInfo, err := os.Lstat(filepath.Dir(paths[0]))
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("OMP context release lineage output directory must be a regular directory")
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return "", "", errors.New("OMP context release lineage output already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", "", errors.New("inspect OMP context release lineage output")
		}
	}
	return paths[0], paths[1], nil
}

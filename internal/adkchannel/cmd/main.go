package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/insajin/autopus-adk/internal/adkchannel"
)

func main() {
	output, err := run(os.Args[1:], os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adk channel receiver: %v\n", err)
		os.Exit(1)
	}
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	writeCommandOutput(os.Stdout, command, output)
}

func writeCommandOutput(writer io.Writer, command, output string) {
	if output == "" {
		return
	}
	if command == "verify-rotation" || command == "verify-rotation-historical" {
		fmt.Fprint(writer, output)
		return
	}
	fmt.Fprintln(writer, output)
}

func run(arguments []string, getenv func(string) string) (string, error) {
	if len(arguments) == 0 {
		return "", fmt.Errorf("command is required")
	}
	switch arguments[0] {
	case "decode":
		return "", runDecode(arguments[1:], getenv)
	case "verify":
		return runVerify(arguments[1:], getenv, false)
	case "verify-update":
		return runVerify(arguments[1:], getenv, true)
	case "verify-rotation":
		return runVerifyRotation(arguments[1:], getenv, false)
	case "verify-rotation-historical":
		return runVerifyRotation(arguments[1:], getenv, true)
	default:
		return "", fmt.Errorf("unknown command %q", arguments[0])
	}
}

func runDecode(arguments []string, getenv func(string) string) error {
	flags := flag.NewFlagSet("decode", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	documentPath := flags.String("document-out", "", "decoded document output")
	signaturePath := flags.String("signature-out", "", "decoded signature output")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *documentPath == "" || *signaturePath == "" {
		return fmt.Errorf("decode requires document-out and signature-out")
	}
	return adkchannel.WriteDispatchInputs(
		getenv("ADK_CHANNEL_DOCUMENT_BASE64"),
		getenv("ADK_CHANNEL_SIGNATURE_BASE64"),
		*documentPath,
		*signaturePath,
	)
}

func runVerify(arguments []string, getenv func(string) string, compareCurrent bool) (string, error) {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	documentPath := flags.String("document", "", "document path")
	signaturePath := flags.String("signature", "", "signature path")
	currentDocumentPath := flags.String("current-document", "", "current document path")
	currentSignaturePath := flags.String("current-signature", "", "current signature path")
	if err := flags.Parse(arguments); err != nil {
		return "", err
	}
	if flags.NArg() != 0 || *documentPath == "" || *signaturePath == "" {
		return "", fmt.Errorf("verify requires document and signature")
	}
	if !compareCurrent && (*currentDocumentPath != "" || *currentSignaturePath != "") {
		return "", fmt.Errorf("current files are valid only for verify-update")
	}
	if (*currentDocumentPath == "") != (*currentSignaturePath == "") {
		return "", fmt.Errorf("current document and signature must be provided together")
	}

	options := adkchannel.Options{
		PublicKeyBase64: getenv("AUTOPUS_ADK_CHANNEL_PUBLIC_KEY"),
		ExpectedKeyID:   getenv("AUTOPUS_ADK_CHANNEL_KEY_ID"),
		Now:             time.Now,
	}
	candidate, err := adkchannel.ReadSignedFiles(*documentPath, *signaturePath)
	if err != nil {
		return "", err
	}
	if !compareCurrent {
		verified, err := adkchannel.Verify(candidate, options)
		if err != nil {
			return "", err
		}
		return verified.Document.Version, nil
	}

	var current *adkchannel.SignedBytes
	if *currentDocumentPath != "" {
		signed, err := adkchannel.ReadSignedFiles(*currentDocumentPath, *currentSignaturePath)
		if err != nil {
			return "", fmt.Errorf("current: %w", err)
		}
		current = &signed
	}
	verified, err := adkchannel.VerifyUpdate(candidate, current, options)
	if err != nil {
		return "", err
	}
	return verified.Document.Version, nil
}

func runVerifyRotation(arguments []string, getenv func(string) string, historical bool) (string, error) {
	command := "verify-rotation"
	if historical {
		command = "verify-rotation-historical"
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	documentPath := flags.String("document", "", "rotation document path")
	signaturePath := flags.String("signature", "", "rotation signature path")
	sourceCommit := flags.String("source-commit", "", "expected source commit")
	sourceTree := flags.String("source-tree", "", "expected source tree")
	tagPublicKeyPath := flags.String("next-tag-public-key", "", "checked-in next tag public key")
	tagFingerprintPath := flags.String("next-tag-fingerprint", "", "checked-in next tag fingerprint")
	promotionPublicKeyPath := flags.String("next-promotion-public-key", "", "checked-in next promotion public key")
	if err := flags.Parse(arguments); err != nil {
		return "", err
	}
	if flags.NArg() != 0 || *documentPath == "" || *signaturePath == "" ||
		*sourceCommit == "" || *sourceTree == "" || *tagPublicKeyPath == "" ||
		*tagFingerprintPath == "" || *promotionPublicKeyPath == "" {
		return "", fmt.Errorf("%s requires document, signature, source commit, source tree, and checked-in pins", command)
	}
	pinFiles := adkchannel.RotationPinFiles{
		NextTagPublicKeyPath:       *tagPublicKeyPath,
		NextTagFingerprintPath:     *tagFingerprintPath,
		NextPromotionPublicKeyPath: *promotionPublicKeyPath,
	}
	options := adkchannel.RotationOptions{
		PublicKeyBase64:      getenv("AUTOPUS_ADK_CHANNEL_PUBLIC_KEY"),
		ExpectedKeyID:        getenv("AUTOPUS_ADK_CHANNEL_KEY_ID"),
		ExpectedSourceCommit: *sourceCommit,
		ExpectedSourceTree:   *sourceTree,
		Now:                  time.Now,
	}
	var verified adkchannel.RotationVerified
	var err error
	if historical {
		verified, err = adkchannel.VerifyRotationFilesHistorical(
			*documentPath,
			*signaturePath,
			pinFiles,
			options,
		)
	} else {
		verified, err = adkchannel.VerifyRotationFiles(
			*documentPath,
			*signaturePath,
			pinFiles,
			options,
		)
	}
	if err != nil {
		return "", err
	}
	return string(verified.CanonicalDocument), nil
}

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
	version, err := run(os.Args[1:], os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adk channel receiver: %v\n", err)
		os.Exit(1)
	}
	if version != "" {
		fmt.Println(version)
	}
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

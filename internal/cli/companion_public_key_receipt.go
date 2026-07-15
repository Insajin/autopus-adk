package cli

import (
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

type companionPublicKeyReceiptOptions struct {
	keyFile              string
	bundleOutput         string
	keyID                string
	issuedAt             string
	expiresAt            string
	handoff              string
	minimumRollbackFloor uint64
}

func newCompanionPublicKeyReceiptCmd() *cobra.Command {
	return newCompanionPublicKeyReceiptCmdWithFaultHook(nil)
}

// newCompanionPublicKeyReceiptCmdWithFaultHook is test-only dependency injection.
// The production command always calls the nil-hook constructor above.
func newCompanionPublicKeyReceiptCmdWithFaultHook(faultHook func(string) error) *cobra.Command {
	var options companionPublicKeyReceiptOptions
	command := &cobra.Command{
		Use:          "public-key-receipt",
		Short:        "Produce a signed public-key handoff receipt",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCompanionPublicKeyReceipt(options, faultHook)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.keyFile, "key-file", "", "0600 Ed25519 private key file")
	flags.StringVar(
		&options.bundleOutput,
		"bundle-output",
		"",
		"Atomically published public-key receipt bundle directory",
	)
	flags.StringVar(&options.keyID, "key-id", "", "Public key identifier")
	flags.StringVar(&options.issuedAt, "issued-at", "", "Canonical UTC issuance time")
	flags.StringVar(&options.expiresAt, "expires-at", "", "Canonical UTC expiry time")
	flags.StringVar(&options.handoff, "handoff", "", "Desktop handoff contract")
	flags.Uint64Var(
		&options.minimumRollbackFloor,
		"minimum-rollback-floor",
		0,
		"Minimum companion rollback floor",
	)
	for _, name := range []string{
		"key-file", "bundle-output", "key-id",
		"issued-at", "expires-at", "handoff", "minimum-rollback-floor",
	} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func runCompanionPublicKeyReceipt(
	options companionPublicKeyReceiptOptions,
	faultHook func(string) error,
) error {
	output, err := resolvePublicKeyReceiptBundle(options.bundleOutput)
	if err != nil {
		return err
	}
	keyPath, err := filepath.Abs(filepath.Clean(options.keyFile))
	if err != nil || keyPath == output.bundlePath {
		return errors.New("key and receipt bundle must be distinct")
	}
	privateKey, err := readPublicKeyReceiptSigningKey(keyPath, faultHook)
	if err != nil {
		return err
	}
	defer clear(privateKey)
	receiptBytes, signature, err := companionmanifest.IssuePublicKeyReceipt(
		companionmanifest.PublicKeyReceiptClaims{
			KeyID:                options.keyID,
			IssuedAt:             options.issuedAt,
			ExpiresAt:            options.expiresAt,
			Handoff:              options.handoff,
			MinimumRollbackFloor: options.minimumRollbackFloor,
		},
		privateKey,
	)
	if err != nil {
		return err
	}
	return writePublicKeyReceiptBundle(output, receiptBytes, signature, faultHook)
}

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func newCompanionOMPContextPromotionKeyIDCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "omp-context-promotion-key-id",
		Short:        "Identify a committed promotion signing key from stdin",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(command *cobra.Command, _ []string) error {
			privateKey, err := readPrivateKey(command.InOrStdin())
			if err != nil {
				return err
			}
			defer clear(privateKey)
			keyID, err := promptlayer.OMPContextPromotionSigningKeyIDV2(privateKey)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(command.OutOrStdout(), keyID); err != nil {
				return errors.New("write OMP context promotion key identity")
			}
			return nil
		},
	}
}

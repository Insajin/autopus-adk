package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

type reviewPrepareOptions struct {
	ContractStdin bool
	JSONOut       bool
}

type reviewPrepareEnvelope struct {
	SchemaVersion string            `json:"schema_version"`
	Command       string            `json:"command"`
	Status        string            `json:"status"`
	GeneratedAt   time.Time         `json:"generated_at"`
	Error         *jsonErrorPayload `json:"error,omitempty"`
	Data          any               `json:"data"`
}

func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Prepare managed typed review contracts",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newReviewPrepareCmd())
	return cmd
}

func newReviewPrepareCmd() *cobra.Command {
	var opts reviewPrepareOptions
	cmd := &cobra.Command{
		Use:          "prepare",
		Short:        "Prepare bounded provider review prompts",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReviewPrepare(cmd, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.ContractStdin, "contract-stdin", false, "Read one managed review contract from stdin")
	cmd.Flags().BoolVar(&opts.JSONOut, "json", false, "Output as JSON")
	return cmd
}

func runReviewPrepare(cmd *cobra.Command, opts reviewPrepareOptions) error {
	if !opts.ContractStdin {
		return finishReviewPrepareError(cmd, opts.JSONOut)
	}
	preparation, err := orchestra.ReadAndPrepareManagedReview(
		cmd.InOrStdin(),
		orchestra.ReviewPrepareMaximumBytes,
	)
	if err != nil {
		return finishReviewPrepareError(cmd, opts.JSONOut)
	}
	if opts.JSONOut {
		return writeReviewPrepareEnvelope(cmd, reviewPrepareEnvelope{
			SchemaVersion: cliJSONSchemaVersion,
			Command:       cmd.CommandPath(),
			Status:        "success",
			GeneratedAt:   time.Now().UTC(),
			Data:          preparation,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "review prepared providers=%d snapshot=%s\n",
		len(preparation.ProviderContracts), preparation.SnapshotDigest)
	return nil
}

func finishReviewPrepareError(cmd *cobra.Command, jsonOutput bool) error {
	if !jsonOutput {
		return orchestra.ErrReviewPrepareInvalid
	}
	envelope := reviewPrepareEnvelope{
		SchemaVersion: cliJSONSchemaVersion,
		Command:       cmd.CommandPath(),
		Status:        "error",
		GeneratedAt:   time.Now().UTC(),
		Error: &jsonErrorPayload{
			Code:    "review_prepare_invalid",
			Message: "review_prepare_invalid",
		},
		Data: map[string]any{"schema_version": orchestra.ReviewPreparationSchemaV1},
	}
	if err := writeReviewPrepareEnvelope(cmd, envelope); err != nil {
		return err
	}
	return &jsonFatalError{cause: orchestra.ErrReviewPrepareInvalid}
}

func writeReviewPrepareEnvelope(cmd *cobra.Command, envelope reviewPrepareEnvelope) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(envelope)
}

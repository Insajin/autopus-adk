package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/delivery"
)

type deliveryDoctorOptions struct {
	RepoScopeRef string
	Phase        string
	JSONOut      bool
	Format       string
}

type deliveryPrepareOptions struct {
	ContractStdin bool
	JSONOut       bool
	Format        string
}

func newDeliveryDoctorCmd() *cobra.Command {
	var opts deliveryDoctorOptions
	cmd := &cobra.Command{
		Use:          "doctor",
		Short:        "Validate a scoped CodeOps phase delivery environment",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeliveryDoctor(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.RepoScopeRef, "repo-scope-ref", "", "Opaque Desktop repository scope reference")
	cmd.Flags().StringVar(&opts.Phase, "phase", "", "Backend-authorized CodeOps phase")
	_ = cmd.MarkFlagRequired("repo-scope-ref")
	_ = cmd.MarkFlagRequired("phase")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func newDeliveryPrepareCmd() *cobra.Command {
	var opts deliveryPrepareOptions
	cmd := &cobra.Command{
		Use:          "prepare",
		Short:        "Prepare one bounded CodeOps phase prompt",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeliveryPrepare(cmd, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.ContractStdin, "contract-stdin", false, "Read one CodeOps execution contract from stdin")
	_ = cmd.MarkFlagRequired("contract-stdin")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runDeliveryDoctor(cmd *cobra.Command, opts deliveryDoctorOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	receipt, err := delivery.Doctor(delivery.DoctorOptions{
		RepoScopeRef: opts.RepoScopeRef,
		Phase:        delivery.Phase(opts.Phase),
	})
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(
				cmd, jsonStatusError, err, delivery.ConvergenceReasonCode(err),
				map[string]any{"schema_version": delivery.DeliveryDoctorSchemaV1}, nil, nil,
			)
		}
		return err
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, receipt, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "delivery ready scope=%s phase=%s harness=%s context=%s\n",
		receipt.RepoScopeRef, receipt.Phase, receipt.HarnessDigest, receipt.ContextDigest)
	return nil
}

func runDeliveryPrepare(cmd *cobra.Command, opts deliveryPrepareOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	if !opts.ContractStdin {
		err = fmt.Errorf("delivery contract stdin is required")
	} else {
		var preparation delivery.Preparation
		preparation, err = delivery.ReadAndPrepare(cmd.InOrStdin(), "", time.Now().UTC())
		if err == nil {
			if jsonMode {
				return writeJSONResult(cmd, jsonStatusOK, preparation, nil, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "delivery prepared scope=%s phase=%s digest=%s\n",
				preparation.RepoScopeRef, preparation.Phase, preparation.PreparationDigest)
			return nil
		}
	}
	if jsonMode {
		return writeJSONResultAndExit(
			cmd, jsonStatusError, err, delivery.ConvergenceReasonCode(err),
			map[string]any{"schema_version": delivery.DeliveryPreparationSchema}, nil, nil,
		)
	}
	return err
}

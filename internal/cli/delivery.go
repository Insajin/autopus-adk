package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/delivery"
)

type deliveryValidateOptions struct {
	File    string
	JSONOut bool
	Format  string
}

type deliveryPlanOptions struct {
	Repository    string
	ProviderMode  string
	OwnerAgentID  string
	CorrelationID string
	RetryBudget   int
	JSONOut       bool
	Format        string
}

func newDeliveryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delivery",
		Short: "Validate ADK supervised delivery contracts",
	}
	cmd.AddCommand(newDeliveryValidateCmd())
	cmd.AddCommand(newDeliveryPlanCmd())
	return cmd
}

func newDeliveryValidateCmd() *cobra.Command {
	var opts deliveryValidateOptions
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a codeops.phase_result.v1 envelope",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeliveryValidate(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.File, "file", "", "Phase result JSON file")
	_ = cmd.MarkFlagRequired("file")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func newDeliveryPlanCmd() *cobra.Command {
	var opts deliveryPlanOptions
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Build a local dry-run supervised delivery phase plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeliveryPlan(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Repository, "repository", ".", "Repository path or identifier")
	cmd.Flags().StringVar(&opts.ProviderMode, "provider-mode", string(delivery.ProviderCodexSubscriptionInteractive), "Provider mode")
	cmd.Flags().StringVar(&opts.OwnerAgentID, "owner-agent-id", delivery.DefaultOwnerAgentID, "Owner agent id")
	cmd.Flags().StringVar(&opts.CorrelationID, "correlation-id", delivery.DefaultCorrelationID, "Correlation id")
	cmd.Flags().IntVar(&opts.RetryBudget, "retry-budget", delivery.DefaultRetryBudget, "Retry budget per phase")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runDeliveryValidate(cmd *cobra.Command, opts deliveryValidateOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(opts.File)
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(cmd, jsonStatusError, err, "delivery_validate_read_failed", map[string]any{"file": opts.File}, nil, nil)
		}
		return err
	}
	envelope, err := delivery.ValidatePhaseResultJSON(data)
	result := map[string]any{
		"schema_version": delivery.PhaseResultSchemaV1,
		"valid":          err == nil,
		"file":           opts.File,
		"phase":          envelope.Phase,
		"status":         envelope.Status,
	}
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(cmd, jsonStatusError, err, "delivery_validate_failed", result, nil, nil)
		}
		return err
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, result, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "delivery envelope valid phase=%s status=%s\n", envelope.Phase, envelope.Status)
	return nil
}

func runDeliveryPlan(cmd *cobra.Command, opts deliveryPlanOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	plan, err := delivery.BuildDryRunPlan(delivery.PlanOptions{
		Repository:    opts.Repository,
		ProviderMode:  delivery.ProviderMode(opts.ProviderMode),
		OwnerAgentID:  opts.OwnerAgentID,
		CorrelationID: opts.CorrelationID,
		RetryBudget:   opts.RetryBudget,
	})
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(cmd, jsonStatusError, err, "delivery_plan_failed", plan, nil, nil)
		}
		return err
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, plan, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s workflow=%s phases=%d provider_mode=%s\n", plan.SchemaVersion, plan.WorkflowMode, len(plan.Phases), plan.ProviderMode)
	return nil
}

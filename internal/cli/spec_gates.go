package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/spec/gates"
)

// newSpecGatesCmd computes gate applicability for a SPEC change set and
// records gate evidence for exact-input reuse.
func newSpecGatesCmd() *cobra.Command {
	var (
		changed          string
		base             string
		jsonOutput       bool
		maxAge           time.Duration
		referenceMissing bool
	)

	cmd := &cobra.Command{
		Use:   "gates <SPEC-ID|SPEC_DIR>",
		Short: "Decide gate applicability and evidence reuse for a SPEC change set",
		Long: `Classifies the change set deterministically, decides which gates are
required, reusable, not_applicable, or blocked, and writes
{SPEC_DIR}/gate-applicability.json. Mandatory safety gates are never
not_applicable. Previously recorded evidence (see "gates record") is reused
only when its exact input closure still matches the current tree.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveGatesTarget(args[0])
			if err != nil {
				return err
			}
			paths, err := resolveGatesChangeSet(target.Root, changed, base)
			if err != nil {
				return err
			}
			cfg, err := config.Load(target.Root)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			prior, err := gates.LoadPriorEvidence(target.Root, target.SpecDir)
			if err != nil {
				return fmt.Errorf("load gate evidence: %w", err)
			}
			receipt := gates.Decide(gates.DecisionInput{
				SpecID:                     target.SpecID,
				Classification:             gates.Classify(paths, cfg.Design.UIFileGlobs),
				AnnotationReferenceMissing: referenceMissing,
				Prior:                      prior,
				Now:                        time.Now(),
				MaxAge:                     maxAge,
			})
			receiptPath, err := gates.WriteApplicability(target.SpecDir, receipt)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeGatesJSON(cmd.OutOrStdout(), receipt)
			}
			printGateDecisions(cmd.OutOrStdout(), receipt, receiptPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&changed, "changed", "", "comma-separated changed paths relative to the project root (default: git working tree changes)")
	cmd.Flags().StringVar(&base, "base", "", "git ref to diff against when --changed is not given (default HEAD)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the applicability receipt as JSON")
	cmd.Flags().DurationVar(&maxAge, "max-age", gates.DefaultMaxAge, "maximum age of evidence eligible for reuse")
	cmd.Flags().BoolVar(&referenceMissing, "annotation-reference-missing", false, "mark the annotation gate blocked because the @AX reference source is absent")

	cmd.AddCommand(newSpecGatesRecordCmd())
	return cmd
}

func newSpecGatesRecordCmd() *cobra.Command {
	var (
		gate        string
		status      string
		inputs      string
		dynamicDeps string
		command     string
		partial     bool
	)

	cmd := &cobra.Command{
		Use:   "record <SPEC-ID|SPEC_DIR>",
		Short: "Record gate evidence with its exact input closure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(gate) == "" {
				return fmt.Errorf("--gate is required")
			}
			if strings.TrimSpace(status) == "" {
				return fmt.Errorf("--status is required")
			}
			if strings.TrimSpace(inputs) == "" {
				return fmt.Errorf("--inputs is required")
			}
			target, err := resolveGatesTarget(args[0])
			if err != nil {
				return err
			}
			receipt, err := gates.BuildEvidence(target.Root, gates.RecordOptions{
				SpecID:      target.SpecID,
				Gate:        gates.GateID(strings.TrimSpace(gate)),
				Status:      gates.EvidenceStatus(strings.ToLower(strings.TrimSpace(status))),
				Partial:     partial,
				InputGlobs:  splitCommaList(inputs),
				DynamicDeps: splitCommaList(dynamicDeps),
				Command:     command,
				Now:         time.Now(),
			})
			if err != nil {
				return fmt.Errorf("record gate evidence: %w", err)
			}
			path, err := gates.WriteEvidence(target.SpecDir, receipt)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "evidence recorded: %s (%s %s, complete=%t, %d input(s), %d dynamic dep(s), closure %s)\n",
				path, receipt.Gate, receipt.Status, receipt.Complete, len(receipt.Inputs), len(receipt.DynamicDeps), receipt.InputClosureSHA256[:12])
			return nil
		},
	}

	cmd.Flags().StringVar(&gate, "gate", "", "gate id (e.g. build, unit_tests, security)")
	cmd.Flags().StringVar(&status, "status", "", "run outcome: pass, fail, or partial")
	cmd.Flags().StringVar(&inputs, "inputs", "", "comma-separated input globs relative to the project root (** supported)")
	cmd.Flags().StringVar(&dynamicDeps, "dynamic-deps", "", "comma-separated dynamic dependency files (e.g. go.sum)")
	cmd.Flags().StringVar(&command, "command", "", "command that produced the evidence")
	cmd.Flags().BoolVar(&partial, "partial", false, "the run did not cover its full scope; evidence is never reusable")
	return cmd
}

func writeGatesJSON(w io.Writer, receipt gates.ApplicabilityReceipt) error {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("encode receipt: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func printGateDecisions(w io.Writer, receipt gates.ApplicabilityReceipt, receiptPath string) {
	fmt.Fprintf(w, "%s (%s): %d changed path(s)\n", receipt.SpecID, receipt.ChangeClass, len(receipt.ChangedPaths))
	for _, decision := range receipt.Decisions {
		fmt.Fprintf(w, "%s: %s — %s\n", decision.Gate, decision.Applicability, decision.Reason)
	}
	fmt.Fprintf(w, "receipt: %s\n", receiptPath)
}

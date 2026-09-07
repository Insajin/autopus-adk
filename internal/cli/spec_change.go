package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/spec/gates"
)

// errEscalateToFullSpec is returned when the declared change class may not
// take the compact path. The printed decision document names the reasons.
var errEscalateToFullSpec = errors.New(string(gates.DecisionEscalate))

type specChangeOptions struct {
	class       string
	acceptance  []string
	surface     []string
	verify      []string
	changeID    string
	newContract bool
	jsonOutput  bool
	force       bool
}

// newSpecChangeCmd records a compact change contract for low-risk work
// against an existing SPEC, instead of authoring a second four-document set.
func newSpecChangeCmd() *cobra.Command {
	opts := specChangeOptions{}

	cmd := &cobra.Command{
		Use:   "change <SPEC-ID|SPEC_DIR>",
		Short: "Record a compact change contract against an existing SPEC",
		Long: `Writes a single change.md that references an existing SPEC and its
acceptance-criteria ids, instead of a second spec/plan/acceptance/research set.

Low-risk classes (test_only, docs_only, small_ui, bugfix_existing_contract)
take the compact contract by default. security_or_data, multi_domain, and
feature work with a new exported API or contract are high risk: they need the
full SPEC set plus a risk-first integration probe, and this command reports
escalate_to_full_spec with the reason instead of writing a compact contract.
A declared class contradicted by the intended surface escalates the same way.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSpecChange(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.class, "class", "", "declared change class: "+strings.Join(changeKindNames(), ", "))
	cmd.Flags().StringSliceVar(&opts.acceptance, "ac", nil, "acceptance-criteria ids the change is bound to (repeatable, comma-separated)")
	cmd.Flags().StringSliceVar(&opts.surface, "surface", nil, "intended surface: files or directories the change may touch")
	cmd.Flags().StringSliceVar(&opts.verify, "verify", nil, "verification plan entries, one command or oracle each (repeatable)")
	cmd.Flags().StringVar(&opts.changeID, "change-id", "", "write .autopus/specs/CHG-<id>/change.md instead of {SPEC_DIR}/change.md")
	cmd.Flags().BoolVar(&opts.newContract, "new-contract", false, "the change introduces a new exported API or contract (always escalates)")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "print the change contract and its gate applicability as JSON")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite an existing change.md")
	return cmd
}

func executeSpecChange(cmd *cobra.Command, arg string, opts specChangeOptions) error {
	if err := validateSpecChangeFlags(opts); err != nil {
		return err
	}
	target, err := resolveGatesTarget(arg)
	if err != nil {
		return err
	}
	class, err := gates.ParseChangeKind(opts.class)
	if err != nil {
		return err
	}
	acceptance, err := resolveAcceptanceIDs(target.SpecDir, opts.acceptance)
	if err != nil {
		return err
	}
	cfg, err := config.Load(target.Root)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	surface := relativizePaths(target.Root, opts.surface)
	classification := gates.Classify(surface, cfg.Design.UIFileGlobs)
	risk := gates.AssessChange(class, classification, opts.newContract)
	receipt := gates.Decide(gates.DecisionInput{
		SpecID:         target.SpecID,
		Classification: classification,
		Change:         risk,
		Now:            time.Now(),
	})

	contract := specChangeContract{
		Schema:           specChangeSchema,
		SpecID:           target.SpecID,
		SpecPath:         filepath.ToSlash(filepath.Join(target.SpecDir, "spec.md")),
		ChangeID:         strings.TrimSpace(opts.changeID),
		AcceptanceIDs:    acceptance,
		Surface:          classification.Paths,
		VerificationPlan: trimmedList(opts.verify),
		NewContract:      opts.newContract,
		Risk:             risk,
		Gates:            receipt.Decisions,
		GeneratedAt:      receipt.GeneratedAt,
	}

	if risk.Escalated() {
		printSpecChange(cmd, contract, opts.jsonOutput)
		return fmt.Errorf("%w: %s — author the full SPEC set and run the risk-first integration probe",
			errEscalateToFullSpec, strings.Join(risk.Reasons, ", "))
	}

	path, err := writeChangeContract(target, contract, opts.force)
	if err != nil {
		return err
	}
	contract.ContractPath = path
	printSpecChange(cmd, contract, opts.jsonOutput)
	return nil
}

func validateSpecChangeFlags(opts specChangeOptions) error {
	if strings.TrimSpace(opts.class) == "" {
		return fmt.Errorf("--class is required: one of %s", strings.Join(changeKindNames(), ", "))
	}
	if len(trimmedList(opts.acceptance)) == 0 {
		return fmt.Errorf("--ac is required: a compact change contract must name the acceptance criteria it is bound to")
	}
	if len(relativizePaths(".", opts.surface)) == 0 {
		return fmt.Errorf("--surface is required: name the files or directories the change may touch")
	}
	if len(trimmedList(opts.verify)) == 0 {
		return fmt.Errorf("--verify is required: name how each referenced acceptance criterion is verified")
	}
	if id := strings.TrimSpace(opts.changeID); id != "" && !validChangeID(id) {
		return fmt.Errorf("invalid --change-id %q: use letters, digits, '-', and '_' only", opts.changeID)
	}
	return nil
}

// writeChangeContract writes change.md either into the referenced SPEC
// directory or into a sibling CHG-<id> directory that points back at it.
func writeChangeContract(target gatesTarget, contract specChangeContract, force bool) (string, error) {
	dir := target.SpecDir
	if contract.ChangeID != "" {
		dir = filepath.Join(target.Root, ".autopus", "specs", "CHG-"+contract.ChangeID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create change directory: %w", err)
		}
	}
	path := filepath.Join(dir, "change.md")
	if !force {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("%s already exists: pass --force to overwrite", filepath.ToSlash(path))
		}
	}
	if err := os.WriteFile(path, []byte(renderChangeContract(contract)), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", filepath.ToSlash(path), err)
	}
	return filepath.ToSlash(path), nil
}

func changeKindNames() []string {
	names := make([]string, 0, len(gates.ChangeKinds))
	for _, kind := range gates.ChangeKinds {
		names = append(names, string(kind))
	}
	return names
}

func trimmedList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func validChangeID(id string) bool {
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

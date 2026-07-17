package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// errSyncVerifyStrict is the sentinel returned under --strict when any violation
// is reported, mapping to a non-zero process exit.
var errSyncVerifyStrict = errors.New("sync verify: violations reported under --strict")

// newSyncCmd creates the help-only parent "sync" command. It has no bare
// behavior of its own; all work lives under subcommands such as "verify".
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Multi-repo sync helpers (2-phase commit planning)",
		Long:  "Read-only helpers for the multi-repo two-phase (module Phase A / meta Phase B) sync workflow.",
	}
	cmd.AddCommand(newSyncVerifyCmd())
	return cmd
}

// newSyncVerifyCmd creates the read-only "sync verify" subcommand.
func newSyncVerifyCmd() *cobra.Command {
	var (
		dir    string
		specID string
		strict bool
	)

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify the deterministic 2-phase commit plan (read-only)",
		Long: "Classify dirty files across the workspace root and nested repos into a deterministic " +
			"Phase A / Phase B commit plan, warn on boundary violations, and never mutate any repo.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := executeSyncVerify(cmd.OutOrStdout(), dir, specID, strict)
			return err
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Directory to start workspace discovery (default: current directory)")
	cmd.Flags().StringVar(&specID, "spec", "", "Restrict analysis to a SPEC id (SPEC-XXX); splits owned vs unrelated dirty files")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero when any violation is reported")
	return cmd
}

// executeSyncVerify runs the full read-only verification and renders the plan to
// out. It returns the number of warnings and, under strict mode with warnings,
// the strict sentinel error. It performs zero git mutations.
func executeSyncVerify(out io.Writer, dir, specID string, strict bool) (int, error) {
	if specID != "" {
		if err := validateSpecID(specID); err != nil {
			return 0, err
		}
	}
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return 0, fmt.Errorf("cannot resolve working directory: %w", err)
		}
		dir = wd
	}

	metaRoot, err := resolveMetaRoot(dir)
	if err != nil {
		return 0, err
	}
	repos, err := collectDirty(metaRoot)
	if err != nil {
		return 0, err
	}

	modules := moduleSet(repos)
	phaseA, phaseB := classifyPhases(repos)
	warnings := detectViolations(repos, modules)

	var specOwned, specUnrelated []string
	if specID != "" {
		owned, unrelated, found := splitSpecOwnership(repos, specID)
		if !found {
			return 0, fmt.Errorf("sync verify: SPEC %s not found under any .autopus/specs/ tree", specID)
		}
		specOwned, specUnrelated = owned, unrelated
		if len(unrelated) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"WARN  unrelated-mixing: dirty files not owned by %s: %s",
				specID, strings.Join(unrelated, ", ")))
		}
	}
	sort.Strings(warnings)

	fmt.Fprintf(out, "sync verify — %d repo(s), read-only (no git mutations)\n\n", len(repos))
	renderPlan(out, phaseA, phaseB)
	if specID != "" {
		renderSpecSplit(out, specID, specOwned, specUnrelated)
	}
	renderWarnings(out, warnings)

	if strict && len(warnings) > 0 {
		return len(warnings), errSyncVerifyStrict
	}
	return len(warnings), nil
}

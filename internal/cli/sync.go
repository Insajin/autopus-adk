package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

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
		Short: "Sync helpers (commit planning)",
		Long: "Read-only helpers for the sync workflow: single-repo commit planning and the " +
			"multi-repo two-phase (module Phase A / meta Phase B) plan.",
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
		Short: "Verify the deterministic commit plan (read-only)",
		Long: "Classify every dirty path into a deterministic commit plan, warn on boundary " +
			"violations, and never mutate any repo. A multi-repo workspace is split into Phase A " +
			"(module repos) and Phase B (meta root); a single Git repository holding autopus.yaml " +
			"yields one commit group. Generated/runtime and unclassified paths are excluded in both.",
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
			return 0, fmt.Errorf("cannot resolve working directory")
		}
		dir = wd
	}

	topology, err := resolveSyncTopology(dir)
	if err != nil {
		return 0, err
	}
	repos, err := collectTopologyDirty(topology)
	if err != nil {
		return 0, err
	}

	modules := moduleSet(repos)
	classified := classifyWorkspace(repos)
	warnings, err := detectViolations(repos, modules, classified)
	if err != nil {
		return 0, err
	}

	var specOwned, specUnrelated []string
	if specID != "" {
		owned, unrelated, splitErr := splitSpecOwnership(repos, specID, classified)
		if splitErr != nil {
			return 0, splitErr
		}
		specOwned, specUnrelated = owned, unrelated
		classified = filterPlanForOwned(classified, owned)
		if len(unrelated) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"WARN  unrelated-mixing: dirty files not owned by %s: %s",
				specID, displayPaths(unrelated)))
		}
	}
	sort.Strings(warnings)

	fmt.Fprintf(out, "sync verify — %d repo(s), read-only (no git mutations)\n", len(repos))
	fmt.Fprintf(out, "%s\n\n", topology.line())
	if topology.Single {
		renderSingleRepoPlan(out, classified.PhaseB)
	} else {
		renderPlan(out, classified.PhaseA, classified.PhaseB)
	}
	if specID != "" {
		renderSpecSplit(out, specID, specOwned, specUnrelated)
	}
	renderWarnings(out, warnings)

	if strict && len(warnings) > 0 {
		return len(warnings), errSyncVerifyStrict
	}
	return len(warnings), nil
}

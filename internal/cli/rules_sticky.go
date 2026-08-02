package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// stickyRootMarkers is the evidence REQ-STICKYRULE-FIRE-12 accepts as a project
// root: a `.claude` directory and nothing else. The sticky body root and the
// manifest both live under it, so a checkout without one has nothing to inject
// and must say so rather than resolve to a root with no installed surface.
var stickyRootMarkers = []string{".claude"}

// unresolvedRootLine is the single diagnostic the unresolved-root case writes.
// It names no path, so a misconfigured working directory is observable without
// the working directory itself reaching stderr (REQ-STICKYRULE-FIRE-04).
const unresolvedRootLine = "sticky project_root_unresolved"

// newRulesStickyCmd builds the UserPromptSubmit entry point. The generated
// sticky hook invokes this command once before each user turn.
func newRulesStickyCmd() *cobra.Command {
	var event string
	cmd := &cobra.Command{
		Use:           "sticky",
		Short:         "Re-attach sticky rule bodies for a UserPromptSubmit payload read from stdin",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fireStickyRules(event, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			// REQ-STICKYRULE-FIRE-03: the runtime is advisory and must exit zero
			// on every path. A returned error becomes a non-zero exit status, and
			// on UserPromptSubmit that erases the prompt the user just typed.
			return nil
		},
	}
	cmd.Flags().StringVar(&event, "event", rulecond.EventUserPromptSubmit,
		"Hook event name the stdin payload belongs to")
	return cmd
}

// fireStickyRules resolves the project root, the effective cadence, and the
// manifest location, then delegates to the sticky runtime.
//
// REQ-STICKYRULE-FIRE-09: no environment variable, command flag, or field of the
// stdin payload may redirect where the manifest is read from. The root is
// derived only from the process working directory.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: the process-exit barrier at the command boundary — everything StickyFire's own recover cannot see (root resolution, config load, cobra plumbing) runs inside this one.
// @AX:REASON: REQ-STICKYRULE-FIRE-11 is an enforced property, not an assertion: an unrecovered Go panic exits with status 2, which is exactly the status that discards the user's prompt on this event.
func fireStickyRules(event string, stdin io.Reader, stdout, stderr io.Writer) {
	defer func() {
		// Nothing is written to stdout before StickyFire, and StickyFire buffers
		// its payload and writes it once, so recovering here can leave no
		// half-written hook payload behind.
		_ = recover()
	}()

	cwd, err := os.Getwd()
	if err != nil {
		// An unreadable working directory is the same observable condition as a
		// missing ancestor: no root could be resolved from it.
		reportUnresolvedStickyRoot(stderr)
		return
	}
	root, ok := resolveStickyRoot(cwd)
	if !ok {
		reportUnresolvedStickyRoot(stderr)
		return
	}
	// Any residual error stays swallowed. StickyFire already contracts to return
	// nil, and discarding it here keeps that contract from being one refactor
	// away from a non-zero exit.
	_ = rulecond.StickyFire(root, event, stickyCadence(root), stdin, stdout, stderr)
}

// resolveStickyRoot walks up from start to the nearest ancestor holding a
// `.claude` directory (REQ-STICKYRULE-FIRE-12).
//
// The walk is the shared one, so the sticky runtime inherits its two trust
// decisions unchanged: it stops at the home directory without testing it,
// because `~/.claude` exists on most Claude Code installs and accepting it would
// anchor the sticky surface outside any checkout, and it rejects a symlinked
// marker, because a cloned repository can ship `.claude` as a symlink.
func resolveStickyRoot(start string) (string, bool) {
	return resolveRootWithMarkers(start, stickyRootMarkers)
}

// stickyCadence reads the configured prompt cadence for the resolved project.
//
// A config that fails to load resolves to the shipped default rather than to an
// error: this runs before every user turn under the exit-zero contract, so a
// malformed autopus.yaml must degrade to the default cadence instead of
// suppressing the rules the SPEC exists to keep present. Returning the unset
// value leaves the default itself stated once, in ResolveStickyCadence.
func stickyCadence(root string) int {
	// LoadPreview, not Load: a hook that runs before every prompt must not
	// rewrite the project's autopus.yaml as a side effect.
	cfg, err := config.LoadPreview(root)
	if err != nil {
		return 0
	}
	return config.StickyCadence(cfg)
}

// reportUnresolvedStickyRoot writes the one diagnostic line the unresolved-root
// case produces, and no stdout at all.
func reportUnresolvedStickyRoot(stderr io.Writer) {
	if stderr == nil {
		return
	}
	fmt.Fprintln(stderr, unresolvedRootLine)
}

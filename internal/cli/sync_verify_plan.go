package cli

import (
	"fmt"
	"io"
	"strings"
)

// loreReminderLine is the copy-ready reminder appended under each commit step.
// It points at the real enforcement command rather than generating a message.
const loreReminderLine = "    # commit with Lore format (auto check --lore enforces type prefix + sign-off)"

// renderPlan writes the deterministic two-phase commit plan: Phase A module
// repos in alphabetical order, then the Phase B meta repo. Only workspace
// relative paths appear; no absolute paths are emitted.
func renderPlan(out io.Writer, phaseA []phaseGroup, phaseB phaseGroup) {
	fmt.Fprintln(out, "Phase A — module commits:")
	if len(phaseA) == 0 {
		fmt.Fprintln(out, "  (no module changes)")
	}
	for _, g := range phaseA {
		renderGroupActions(out, g)
		fmt.Fprintln(out, loreReminderLine)
	}

	fmt.Fprintln(out, "Phase B — meta commit:")
	if len(phaseB.Files) == 0 {
		fmt.Fprintln(out, "  (no meta changes)")
		return
	}
	renderGroupActions(out, phaseB)
	fmt.Fprintln(out, loreReminderLine)
}

func renderGroupActions(out io.Writer, group phaseGroup) {
	if len(group.AddFiles) > 0 {
		fmt.Fprintf(out, "  git -C %s add -- %s\n", group.RepoPath, strings.Join(group.AddFiles, " "))
	}
	if len(group.UpdateFiles) > 0 {
		fmt.Fprintf(out, "  git -C %s add -u -- %s\n", group.RepoPath, strings.Join(group.UpdateFiles, " "))
	}
	if len(group.StagedOnly) > 0 {
		fmt.Fprintf(out, "  already staged in %s: %s\n", group.RepoPath, displayPaths(group.StagedOnly))
	}
}

// renderSpecSplit prints the --spec owned vs unrelated partition.
func renderSpecSplit(out io.Writer, specID string, owned, unrelated []string) {
	fmt.Fprintf(out, "\n--spec %s ownership:\n", specID)
	fmt.Fprintf(out, "  owned:     %s\n", joinOrNone(owned))
	fmt.Fprintf(out, "  unrelated: %s\n", joinOrNone(unrelated))
}

// renderWarnings prints the deterministic warning block.
func renderWarnings(out io.Writer, warnings []string) {
	if len(warnings) == 0 {
		fmt.Fprintln(out, "\nWarnings: none")
		return
	}
	fmt.Fprintf(out, "\nWarnings (%d):\n", len(warnings))
	for _, w := range warnings {
		fmt.Fprintln(out, w)
	}
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return displayPaths(items)
}

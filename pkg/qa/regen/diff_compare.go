package regen

import (
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// comparePacks returns the ordered set of FieldChange entries describing how the
// synthesized pack (after) differs from the existing pack (before). Fields are
// compared by stable rendered string form so json.Marshal of the diff is
// deterministic. The returned slice is sorted by Field for determinism.
func comparePacks(before, after journey.Pack) []FieldChange {
	changes := make([]FieldChange, 0)
	add := func(field, b, a string) {
		if b != a {
			changes = append(changes, FieldChange{Field: field, Before: b, After: a})
		}
	}
	add("title", before.Title, after.Title)
	add("surface", before.Surface, after.Surface)
	add("adapter.id", before.Adapter.ID, after.Adapter.ID)
	add("lanes", renderList(before.Lanes), renderList(after.Lanes))
	add("command.argv", renderArgv(before.Command.Argv), renderArgv(after.Command.Argv))
	add("command.run", before.Command.Run, after.Command.Run)
	add("command.cwd", before.Command.CWD, after.Command.CWD)
	add("command.timeout", before.Command.Timeout, after.Command.Timeout)
	add("source_refs.source_spec", before.SourceRefs.SourceSpec, after.SourceRefs.SourceSpec)
	add("pass_fail_authority", before.PassFailAuthority, after.PassFailAuthority)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Field < changes[j].Field })
	return changes
}

// renderArgv renders a command argv as the bracket-joined string form used in
// diff Before/After, e.g. ["npm" "run" "test"] -> "[npm run test]".
func renderArgv(argv []string) string {
	return "[" + strings.Join(argv, " ") + "]"
}

// renderList renders a string slice as a bracket-joined form for stable diffing.
func renderList(values []string) string {
	return "[" + strings.Join(values, " ") + "]"
}

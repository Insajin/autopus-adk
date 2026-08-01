package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

const (
	// contentRuleDir is the embedded rule source of truth (content/rules).
	contentRuleDir = "rules"
	// claudeRuleDir is the claude-code destination for always and paths-scoped
	// rules. Hook-fired bodies are relocated to rulecond.BodyRootRelPath so they
	// cost no baseline context.
	claudeRuleDir = ".claude/rules/autopus"
	// defaultHookEvent is the only hook event SPEC-CONDRULE-001 compiles to.
	defaultHookEvent = "PreToolUse"
	// noTriggerSummary marks a rule that carries no trigger field.
	noTriggerSummary = "-"
)

// ruleClassLabels pins the operator-visible classification vocabulary in the CLI
// layer, so the reported labels stay fixed regardless of how pkg/rulecond
// represents a Class internally.
var ruleClassLabels = map[rulecond.Class]string{
	rulecond.ClassAlways:      "always",
	rulecond.ClassPathsScoped: "paths-scoped",
	rulecond.ClassHookFired:   "hook-fired",
}

// newRulesCmd builds the `auto rules` namespace.
func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "rules",
		Short:         "Inspect ADK rule triggers and dispatch conditional rules",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newRulesFireCmd())
	cmd.AddCommand(newRulesListCmd())
	return cmd
}

// newRulesFireCmd builds the PreToolUse dispatcher entry point. The generated
// claude-code hook invokes this command once per matching tool call.
func newRulesFireCmd() *cobra.Command {
	var event string
	cmd := &cobra.Command{
		Use:           "fire",
		Short:         "Inject conditional rule bodies for a hook payload read from stdin",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fireConditionalRules(event, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			// REQ-CONDRULE-FIRE-06: the dispatcher is advisory. It never denies,
			// never asks, and never leaves a non-zero exit code behind, so a
			// dispatcher fault can never block the tool call it observed.
			return nil
		},
	}
	cmd.Flags().StringVar(&event, "event", defaultHookEvent,
		"Hook event name the stdin payload belongs to")
	return cmd
}

// maxRootSearchDepth bounds the upward walk for the project root, so a
// pathological directory chain cannot turn root resolution into a long walk on
// every tool call.
const maxRootSearchDepth = 64

// projectRootMarkers are the directory entries that identify a project root.
// `.claude` is the surface the manifest itself lives under; `.git` covers a
// checkout whose harness has not been generated yet.
var projectRootMarkers = []string{".claude", ".git"}

// fireConditionalRules resolves the manifest at its fixed project-relative
// location and delegates to the dispatcher.
//
// REQ-CONDRULE-FIRE-11: no environment variable, no command flag, and no field
// of the stdin payload may redirect where the manifest is read from. The root
// is derived only from the process working directory, by walking up to the
// nearest enclosing project marker.
func fireConditionalRules(event string, stdin io.Reader, stdout, stderr io.Writer) {
	cwd, err := os.Getwd()
	if err != nil {
		// Fail open: an unreadable working directory is benign absence.
		return
	}
	root, ok := resolveProjectRoot(cwd)
	if !ok {
		// Fail open: outside any project there is no manifest to read.
		return
	}
	// REQ-CONDRULE-FIRE-03 and FIRE-09 are enforced inside Fire, which reports
	// fail-closed suppressions on stderr and keeps every other matched rule
	// injected. Any residual error stays swallowed to preserve exit zero.
	_ = rulecond.Fire(root, event, stdin, stdout, stderr)
}

// resolveProjectRoot walks up from start to the nearest directory holding a
// project marker.
//
// Trust decision: the working directory is an implicit input the invoking
// process chooses, so anchoring the manifest at the bare CWD lets any directory
// containing a crafted `.claude/hooks/autopus/` tree be read as if it were the
// project. Requiring a marker binds the manifest to a real checkout, and the
// walk stops at the home directory WITHOUT testing it: `~/.claude` exists on
// most Claude Code installs, and treating it as a project root would restore
// exactly the redirection this check removes. The filesystem root and
// maxRootSearchDepth bound the walk otherwise.
func resolveProjectRoot(start string) (string, bool) {
	home, _ := os.UserHomeDir()
	dir := filepath.Clean(start)

	for depth := 0; depth < maxRootSearchDepth; depth++ {
		if home != "" && dir == filepath.Clean(home) {
			return "", false
		}
		for _, marker := range projectRootMarkers {
			// os.Lstat, not os.Stat: a symlinked marker is not evidence of a
			// project root. A cloned repository can ship `.claude` as a symlink
			// (git mode 120000 survives the clone), and accepting it here would
			// anchor the whole conditional surface outside the checkout. Falling
			// through to the next marker keeps a repo whose `.git` is real
			// resolvable, and rulecond's trusted-root contract then refuses the
			// symlinked component when the manifest is read.
			info, err := os.Lstat(filepath.Join(dir, marker))
			if err == nil && info.Mode()&os.ModeSymlink == 0 {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
	return "", false
}

// newRulesListCmd builds the REQ-CONDRULE-OBS-01 inspection command.
func newRulesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List every rule with its classification, trigger, and compiled destination",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows, err := collectRuleListRows()
			if err != nil {
				return err
			}
			return renderRuleListRows(cmd.OutOrStdout(), rows)
		},
	}
}

// ruleListRow is one printed `auto rules list` row.
type ruleListRow struct {
	Name        string
	Class       string
	Trigger     string
	Destination string
}

// collectRuleListRows parses every embedded rule, validates it, and derives its
// classification, trigger summary, and compiled claude-code destination.
func collectRuleListRows() ([]ruleListRow, error) {
	entries, err := fs.ReadDir(contentfs.FS, contentRuleDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded rules: %w", err)
	}

	rows := make([]ruleListRow, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, readErr := fs.ReadFile(contentfs.FS, path.Join(contentRuleDir, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read rule %s: %w", entry.Name(), readErr)
		}
		rule, parseErr := rulecond.ParseRule(entry.Name(), raw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse rule %s: %w", entry.Name(), parseErr)
		}
		if validateErr := rulecond.Validate(rule); validateErr != nil {
			return nil, fmt.Errorf("invalid rule %s: %w", entry.Name(), validateErr)
		}
		rows = append(rows, newRuleListRow(entry.Name(), rule))
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows, nil
}

// newRuleListRow derives one row. The rule name follows the source file name,
// because that is what names the emitted file on every platform.
func newRuleListRow(sourceFile string, rule *rulecond.Rule) ruleListRow {
	name := strings.TrimSuffix(sourceFile, ".md")
	class := rulecond.Classify(rule)
	label, known := ruleClassLabels[class]
	if !known {
		label = "unknown"
	}
	return ruleListRow{
		Name:        name,
		Class:       label,
		Trigger:     ruleTriggerSummary(class, rule),
		Destination: ruleDestination(class, name),
	}
}

// ruleTriggerSummary renders what fires a rule: the tool scopes for a hook-fired
// rule, the path globs for a paths-scoped rule, nothing otherwise.
func ruleTriggerSummary(class rulecond.Class, rule *rulecond.Rule) string {
	var fields []string
	switch class {
	case rulecond.ClassHookFired:
		fields = rule.Scopes
	case rulecond.ClassPathsScoped:
		fields = rule.Globs
	}
	if len(fields) == 0 {
		return noTriggerSummary
	}
	return strings.Join(fields, ",")
}

// ruleDestination reports where the claude-code compiler writes the rule.
func ruleDestination(class rulecond.Class, name string) string {
	if class == rulecond.ClassHookFired {
		return path.Join(filepath.ToSlash(rulecond.BodyRootRelPath), name+".md")
	}
	return path.Join(claudeRuleDir, name+".md")
}

// renderRuleListRows prints one aligned row per rule followed by the
// classification totals.
func renderRuleListRows(out io.Writer, rows []ruleListRow) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "RULE\tCLASS\tTRIGGER\tCLAUDE-CODE-DESTINATION"); err != nil {
		return err
	}

	counts := make(map[string]int, len(ruleClassLabels))
	for _, row := range rows {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n",
			row.Name, row.Class, row.Trigger, row.Destination); err != nil {
			return err
		}
		counts[row.Class]++
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	_, err := fmt.Fprintf(out, "\n%d rules: %d always, %d paths-scoped, %d hook-fired\n",
		len(rows), counts["always"], counts["paths-scoped"], counts["hook-fired"])
	return err
}

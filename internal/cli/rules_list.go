package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// newRulesListCmd builds the REQ-CONDRULE-OBS-01 and REQ-STICKYRULE-OBS-01
// inspection command.
func newRulesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List every rule with its classification, sticky flag, trigger, and compiled destination",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows, err := collectRuleListRows()
			if err != nil {
				return err
			}
			cadence, err := effectiveStickyCadence()
			if err != nil {
				return err
			}
			return renderRuleListRows(cmd.OutOrStdout(), rows, cadence)
		},
	}
}

// ruleListRow is one printed `auto rules list` row.
type ruleListRow struct {
	Name        string
	Class       string
	Sticky      bool
	Trigger     string
	Destination string
}

// collectRuleListRows parses every embedded rule, validates it, and derives its
// classification, sticky flag, trigger summary, and compiled claude-code
// destination.
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
		Sticky:      rulecond.IsSticky(rule),
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

// ruleDestination reports where the claude-code compiler writes the rule. Both
// relocated classes land under the conditional body root; what re-attaches the
// body differs (a dispatcher match versus a skill reference), but the operator
// looking for the file needs the same path.
func ruleDestination(class rulecond.Class, name string) string {
	if rulecond.RelocatesBody(class) {
		return path.Join(filepath.ToSlash(rulecond.BodyRootRelPath), name+".md")
	}
	return path.Join(claudeRuleDir, name+".md")
}

// effectiveStickyCadence resolves the cadence an operator would actually get for
// the project the command runs in.
//
// Reporting the configured integer would be misleading where the key is absent,
// zero, or negative, because those all resolve to the shipped default at
// dispatch time (REQ-STICKYRULE-MAP-01). The resolution is therefore done with
// the same function the runtime uses. A config that fails to parse is surfaced
// as an error rather than silently reported as the default: `auto rules list` is
// an inspection command, and a fabricated value is worse than a refusal.
func effectiveStickyCadence() (int, error) {
	dir, err := os.Getwd()
	if err != nil {
		return rulecond.DefaultStickyCadence, nil
	}
	if root, ok := resolveStickyRoot(dir); ok {
		dir = root
	}
	// LoadPreview, not Load: an inspection command must not rewrite the
	// project's autopus.yaml as a side effect of being run.
	cfg, err := config.LoadPreview(dir)
	if err != nil {
		return 0, fmt.Errorf("resolve sticky cadence: %w", err)
	}
	return rulecond.ResolveStickyCadence(config.StickyCadence(cfg)), nil
}

// renderRuleListRows prints one aligned row per rule followed by the
// classification totals and the effective sticky cadence.
func renderRuleListRows(out io.Writer, rows []ruleListRow, cadence int) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "RULE\tCLASS\tSTICKY\tTRIGGER\tCLAUDE-CODE-DESTINATION"); err != nil {
		return err
	}

	counts := make(map[string]int, len(ruleClassLabels))
	sticky := 0
	for _, row := range rows {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
			row.Name, row.Class, strconv.FormatBool(row.Sticky),
			row.Trigger, row.Destination); err != nil {
			return err
		}
		counts[row.Class]++
		if row.Sticky {
			sticky++
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "\n%d rules: %d always, %d paths-scoped, %d hook-fired, %d skill-scoped\n",
		len(rows), counts["always"], counts["paths-scoped"], counts["hook-fired"],
		counts["skill-scoped"]); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "%d sticky, re-attached on an effective cadence of %d prompts\n",
		sticky, cadence)
	return err
}

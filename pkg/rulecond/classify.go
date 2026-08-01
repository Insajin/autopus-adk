package rulecond

import (
	"fmt"
	"regexp"
	"strings"
)

// Class is the classification of a rule. The three values partition every rule
// exactly once (REQ-CONDRULE-SCHEMA-02).
type Class string

const (
	// ClassAlways is a rule with no trigger field. It keeps loading at session
	// start and its emitted content is unchanged.
	ClassAlways Class = "always"
	// ClassPathsScoped is a rule with `globs` and no `condition`. Claude Code
	// loads it natively when a matching file is read.
	ClassPathsScoped Class = "paths-scoped"
	// ClassHookFired is a rule with a `condition` and a tool `scope`. Its body
	// leaves baseline context and the dispatcher injects it on a match.
	ClassHookFired Class = "hook-fired"
)

// Classify returns the single class of a rule. A `condition` with a tool scope
// wins over `globs`, because the condition is the narrower trigger.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: total 3-way partition that decides where every rule lands — the claude adapter's relocates(), the compiler, and `auto rules list` all branch on it.
// @AX:REASON: A rule that classifies differently between call sites is emitted twice or not at all; the condition-over-globs precedence is what keeps that decision single-valued.
func Classify(rule *Rule) Class {
	if rule == nil {
		return ClassAlways
	}
	if len(rule.Conditions) > 0 && len(toolTriggers(rule)) > 0 {
		return ClassHookFired
	}
	if len(rule.Globs) > 0 && len(rule.Conditions) == 0 {
		return ClassPathsScoped
	}
	return ClassAlways
}

// Triggers returns the parsed tool scopes of a rule, deduplicated by event and
// matcher pair so a rule contributes one dispatcher entry per distinct pair.
func Triggers(rule *Rule) []Trigger {
	parsed := toolTriggers(rule)
	seen := map[string]bool{}
	out := make([]Trigger, 0, len(parsed))
	for _, trigger := range parsed {
		key := trigger.Event + "\x00" + trigger.Matcher
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trigger)
	}
	return out
}

// toolTriggers parses every scope entry, dropping the ones that do not parse.
// Validate is what reports those as errors; classification stays total.
func toolTriggers(rule *Rule) []Trigger {
	if rule == nil {
		return nil
	}
	var out []Trigger
	for _, scope := range rule.Scopes {
		trigger, err := ParseScope(scope)
		if err != nil {
			continue
		}
		out = append(out, trigger)
	}
	return out
}

// Validate rejects a rule whose trigger fields cannot compile into a working
// trigger. Every error names the offending rule file and the offending field.
func Validate(rule *Rule) error {
	if rule == nil {
		return fmt.Errorf("rule is nil")
	}
	name := rule.ident()

	for _, condition := range rule.Conditions {
		if looksLikeGlob(condition) {
			return fmt.Errorf(
				"rule %s: field condition holds the file glob %q; a file glob belongs in globs, not condition",
				name, condition)
		}
		if _, err := regexp.Compile(condition); err != nil {
			return fmt.Errorf("rule %s: field condition is not a valid regex: %w", name, err)
		}
	}

	for _, scope := range rule.Scopes {
		if _, err := ParseScope(scope); err != nil {
			return fmt.Errorf("rule %s: field scope is invalid: %w", name, err)
		}
	}

	if len(rule.Conditions) > 0 && len(toolTriggers(rule)) == 0 {
		return fmt.Errorf(
			"rule %s: field condition requires a tool scope such as `scope: tool:bash`", name)
	}
	return nil
}

// ident names a rule in an error, preferring the source file name.
func (r *Rule) ident() string {
	if r.SourceFile != "" {
		return r.SourceFile
	}
	return r.Name
}

// looksLikeGlob reports whether a condition value reads as a file glob rather
// than a regex. The omp parser coerces a glob-shaped condition into an
// `tool:edit(...)` scope, which silently changes the rule meaning across
// tracks, so the ADK refuses the value instead (REQ-CONDRULE-SCHEMA-03).
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-CONDRULE-001: domain heuristic, not a parser — the four branches (`**`, no wildcard, RE2-uncompilable, wildcard plus `/`) encode which shapes are glob-only.
func looksLikeGlob(value string) bool {
	if value == "" {
		return false
	}
	// `**` is a glob-only construct: in RE2 it is an invalid repetition.
	if strings.Contains(value, "**") {
		return true
	}
	// Without a wildcard there is nothing glob-shaped left to detect.
	if !strings.ContainsAny(value, "*?") {
		return false
	}
	// A wildcard that RE2 cannot parse is a bare glob quantifier, as in `*.go`.
	if _, err := regexp.Compile(value); err != nil {
		return true
	}
	// A wildcard plus a path separator and no regex-only construct reads as a
	// path pattern, as in `src/*.ts`.
	return strings.Contains(value, "/") && !strings.ContainsAny(value, `\^$+|()[]{}`)
}

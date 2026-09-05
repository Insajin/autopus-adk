package rulecond

import (
	"fmt"
	"regexp"
	"strings"
)

// Class is the classification of a rule. The four values partition every rule
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
	// ClassSkillScoped is a rule with `skillScoped: true` and no other trigger
	// field. Its body leaves baseline context like a hook-fired body, but no
	// dispatcher or manifest entry is produced: the `/auto` workflow skills and
	// agents that reference the relocated path are the only thing that loads it.
	ClassSkillScoped Class = "skill-scoped"
)

// Classify returns the single class of a rule. A `condition` with a tool scope
// wins over `skillScoped` and `globs`, because the condition is the narrower
// trigger, and `skillScoped` wins over `globs` because a rule the compiler
// relocates has no baseline file left for a native `paths:` list to sit in.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: total 4-way partition that decides where every rule lands — the claude adapter's relocates(), the compiler, and `auto rules list` all branch on it.
// @AX:REASON: A rule that classifies differently between call sites is emitted twice or not at all; the condition-over-skillScoped-over-globs precedence is what keeps that decision single-valued.
func Classify(rule *Rule) Class {
	if rule == nil {
		return ClassAlways
	}
	if len(rule.Conditions) > 0 && len(toolTriggers(rule)) > 0 {
		return ClassHookFired
	}
	if rule.SkillScoped {
		return ClassSkillScoped
	}
	if len(rule.Globs) > 0 && len(rule.Conditions) == 0 {
		return ClassPathsScoped
	}
	return ClassAlways
}

// RelocatesBody reports whether a class moves its rule body out of
// ClaudeRulesRelDir into BodyRootRelPath, costing no baseline context.
//
// Hook-fired and skill-scoped bodies differ only in what re-attaches them: a
// dispatcher match for the former, a skill reference for the latter. Every
// consumer that cares about placement rather than about triggering — the
// compiler, IsSticky, the update prune, `auto rules list` — asks this instead
// of enumerating classes, so a new relocated class cannot land in one of them
// and be forgotten in the others.
func RelocatesBody(class Class) bool {
	return class == ClassHookFired || class == ClassSkillScoped
}

// IsSticky reports whether a rule is re-attached on the SPEC-STICKYRULE-001
// prompt cadence.
//
// Sticky is an orthogonal boolean on top of the class partition, never a class
// of its own (REQ-STICKYRULE-SCHEMA-01): an `always` or `paths-scoped` rule
// keeps its baseline load placement and merely gains a periodic restatement. A
// rule whose class relocates its body can never be sticky, because
// RelocatesBody moves that body to BodyRootRelPath while this SPEC resolves
// sticky bodies from ClaudeRulesRelDir only. Validate refuses both
// combinations outright, so this predicate and the validator agree on the same
// unrepresentable cases.
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: paired invariant — this predicate and the REQ-STICKYRULE-SCHEMA-03 branch of Validate below must exclude the same classes; if they drift, a sticky relocated rule compiles to a manifest entry whose body the runtime can never resolve.
func IsSticky(rule *Rule) bool {
	if rule == nil {
		return false
	}
	return rule.AlwaysApply && !RelocatesBody(Classify(rule))
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

	// The refusals below keep `skillScoped` a standalone trigger instead of a
	// modifier that silently loses to a neighbouring field. Each combination is
	// unrepresentable rather than merely redundant: a skill-scoped rule is
	// relocated with no dispatcher entry and no baseline file, so a `condition`
	// would classify as hook-fired and be injected by a matcher the author did
	// not ask for, a `globs` list would have no baseline file left to carry its
	// native `paths:` frontmatter, and `alwaysApply` would name a sticky body
	// the runtime resolves under ClaudeRulesRelDir and can never find.
	if rule.SkillScoped {
		switch {
		case len(rule.Conditions) > 0:
			return fmt.Errorf(
				"rule %s: field skillScoped cannot combine with field condition, because a "+
					"skill-scoped rule is loaded by the skill that references it and gets no "+
					"dispatcher entry", name)
		case len(rule.Globs) > 0:
			return fmt.Errorf(
				"rule %s: field skillScoped cannot combine with field globs, because a "+
					"skill-scoped body is relocated out of the baseline rule directory that "+
					"carries the native paths: list", name)
		case rule.AlwaysApply:
			return fmt.Errorf(
				"rule %s: field skillScoped cannot combine with field alwaysApply, because a "+
					"skill-scoped body is relocated out of the sticky body root", name)
		}
	}

	// REQ-STICKYRULE-SCHEMA-03: the combination is unrepresentable rather than
	// merely undesirable. A hook-fired body lives under BodyRootRelPath, and the
	// sticky runtime resolves every body under ClaudeRulesRelDir, so a sticky
	// hook-fired rule would compile to a manifest entry whose body the runtime
	// can never find. Refusing it here is what keeps that from degrading into a
	// rule that silently stops re-attaching.
	if rule.AlwaysApply && Classify(rule) == ClassHookFired {
		return fmt.Errorf(
			"rule %s: field alwaysApply cannot combine with the hook-fired classification, "+
				"because a hook-fired body is relocated out of the sticky body root", name)
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

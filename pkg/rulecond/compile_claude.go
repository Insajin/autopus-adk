package rulecond

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// Dispatcher hook shape. One entry per distinct event and matcher pair keeps
// the per-tool-call cost at a single process spawn (REQ-CONDRULE-COMPILE-03).
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: single source of truth for the dispatcher hook shape — pkg/content/hooks_conditional.go::ConditionalDispatcherHooks derives the settings.json registration from CompiledClaude.Hooks built from these constants.
// @AX:REASON: Registration is no longer restated in pkg/content, so changing a command or timeout here is the only edit point and propagates to every registered event and matcher pair.
const (
	dispatcherType    = "command"
	dispatcherCommand = "auto rules fire --event "
	dispatcherTimeout = 10
)

// CompiledClaude is the claude-code compilation output of the triggered rules.
//
// Only paths-scoped and hook-fired rules are claimed here. An `always` rule is
// deliberately absent from every field, because it must keep flowing through
// the existing verbatim copy path that keeps its emitted bytes unchanged
// (REQ-CONDRULE-SCHEMA-02).
type CompiledClaude struct {
	// RuleFiles are paths-scoped rules that stay in the baseline rule
	// directory, carrying a native `paths:` frontmatter list.
	RuleFiles []adapter.FileMapping
	// Bodies are hook-fired rule bodies relocated into the conditional body
	// root, so they cost no baseline context.
	Bodies []adapter.FileMapping
	// Manifest is the compiled conditional manifest. It is always produced,
	// and holds an empty rule list when no rule is hook-fired.
	Manifest adapter.FileMapping
	// Hooks holds one dispatcher entry per distinct hook event and matcher.
	Hooks []adapter.HookConfig
}

// CompileClaude compiles rules for claude-code. It fails rather than emitting a
// manifest entry the dispatcher would later refuse, and the error names the
// offending rule (REQ-CONDRULE-COMPILE-06).
func CompileClaude(rules []*Rule) (*CompiledClaude, error) {
	out := &CompiledClaude{}
	var manifestRules []ManifestRule
	var triggers []Trigger
	seen := map[string]bool{}

	for _, rule := range rules {
		switch Classify(rule) {
		case ClassPathsScoped:
			out.RuleFiles = append(out.RuleFiles, ruleFileMapping(rule))
		case ClassHookFired:
			body := []byte(rule.Body)
			if err := checkCompiledBody(rule.Name, body); err != nil {
				return nil, err
			}
			out.Bodies = append(out.Bodies, bodyMapping(rule.Name, body))
			for _, trigger := range Triggers(rule) {
				manifestRules = append(manifestRules, manifestEntry(rule, trigger))
				key := trigger.Event + "\x00" + trigger.Matcher
				if !seen[key] {
					seen[key] = true
					triggers = append(triggers, trigger)
				}
			}
		case ClassAlways:
		}
	}

	blob, err := encodeManifest(manifestRules)
	if err != nil {
		return nil, err
	}
	out.Manifest = mapping(filepath.FromSlash(ManifestRelPath), blob)
	out.Hooks = dispatcherHooks(triggers)
	sortMappings(out.RuleFiles)
	sortMappings(out.Bodies)
	return out, nil
}

// manifestEntry records one rule under one trigger. The body location is stored
// relative to the conditional body root (REQ-CONDRULE-COMPILE-07).
func manifestEntry(rule *Rule, trigger Trigger) ManifestRule {
	conditions := make([]string, len(rule.Conditions))
	copy(conditions, rule.Conditions)
	return ManifestRule{
		Name:       rule.Name,
		Event:      trigger.Event,
		Matcher:    trigger.Matcher,
		Conditions: conditions,
		Body:       rule.Name + mdSuffix,
	}
}

// dispatcherHooks builds one hook entry per distinct event and matcher pair,
// ordered so regeneration is stable.
func dispatcherHooks(triggers []Trigger) []adapter.HookConfig {
	ordered := make([]Trigger, len(triggers))
	copy(ordered, triggers)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Event != ordered[j].Event {
			return ordered[i].Event < ordered[j].Event
		}
		return ordered[i].Matcher < ordered[j].Matcher
	})

	hooks := make([]adapter.HookConfig, 0, len(ordered))
	for _, trigger := range ordered {
		hooks = append(hooks, adapter.HookConfig{
			Event:   trigger.Event,
			Matcher: trigger.Matcher,
			Type:    dispatcherType,
			Command: dispatcherCommand + trigger.Event,
			Timeout: dispatcherTimeout,
		})
	}
	return hooks
}

// bodyMapping relocates one hook-fired rule body into the conditional body root.
func bodyMapping(name string, body []byte) adapter.FileMapping {
	target := filepath.Join(filepath.FromSlash(BodyRootRelPath), name+mdSuffix)
	return mapping(target, body)
}

// ruleFileMapping renders a paths-scoped rule for the baseline rule directory.
func ruleFileMapping(rule *Rule) adapter.FileMapping {
	target := filepath.Join(filepath.FromSlash(ClaudeRulesRelDir), rule.Name+mdSuffix)
	return mapping(target, []byte(renderPathsScoped(rule)))
}

// mapping builds a FileMapping with the checksum the installer compares.
func mapping(target string, content []byte) adapter.FileMapping {
	return adapter.FileMapping{
		TargetPath:      target,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        adapter.Checksum(string(content)),
		Content:         content,
	}
}

// sortMappings orders mappings by target path so regeneration is stable.
func sortMappings(files []adapter.FileMapping) {
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].TargetPath < files[j].TargetPath
	})
}

// renderPathsScoped translates `globs` into a native Claude Code `paths:`
// frontmatter list (REQ-CONDRULE-COMPILE-01) and preserves `interruptMode` and
// `astCondition` verbatim (REQ-CONDRULE-SCHEMA-04).
func renderPathsScoped(rule *Rule) string {
	var b strings.Builder
	b.WriteString("---\n")
	writeScalar(&b, "name", rule.Name)
	writeScalar(&b, "description", rule.Description)
	writeScalar(&b, "category", rule.Category)
	if len(rule.Globs) > 0 {
		b.WriteString("paths:\n")
		for _, glob := range rule.Globs {
			b.WriteString("  - " + yamlScalar(glob) + "\n")
		}
	}
	if rule.AlwaysApply {
		b.WriteString("alwaysApply: true\n")
	}
	writeScalar(&b, "interruptMode", rule.InterruptMode)
	writeScalar(&b, "astCondition", rule.AstCondition)
	b.WriteString("---\n\n")
	b.WriteString(rule.Body)
	return b.String()
}

// writeScalar emits one frontmatter key, skipping an empty value.
func writeScalar(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	b.WriteString(key + ": " + yamlScalar(value) + "\n")
}

// yamlScalar renders a single-line YAML scalar, quoting only when the value
// would otherwise change meaning.
func yamlScalar(value string) string {
	if value == "" {
		return `""`
	}
	if strings.TrimSpace(value) != value || strings.ContainsAny(value, ":#{}[]&*!|>'\"%@`,\n\\") {
		return strconv.Quote(value)
	}
	return value
}

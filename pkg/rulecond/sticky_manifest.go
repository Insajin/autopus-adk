package rulecond

import "sort"

// StickyRule is one compiled sticky rule: a rule the SPEC-STICKYRULE-001
// runtime re-attaches on a fixed prompt cadence.
//
// The sticky set is deliberately recorded inside the SPEC-CONDRULE-001 manifest
// at ManifestRelPath rather than in a manifest of its own. A second generated
// file under `.claude/hooks/autopus/` would sit outside the
// `.claude/hooks/autopus/conditional` prefix that
// internal/cli/check_rules_hygiene.go maps back to `content/rules/`, so drift
// detection and the adapter parity classifier would both report it as an orphan
// nobody owns.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: the sticky manifest contract shared by CompileClaude and StickyFire — the compiler writes Body relative to ClaudeRulesRelDir and the runtime resolves it under exactly that root.
// @AX:REASON: Recording an absolute or root-relative Body here would hand the runtime a location its containment frame cannot bound.
type StickyRule struct {
	// Name is the rule name. It is the injection header, the ordering key, and
	// the only identifier a fail-closed suppression may report.
	Name string `json:"name"`
	// Body is the rule body location relative to the sticky body root
	// ClaudeRulesRelDir, which is the baseline directory Claude Code already
	// loads an `always` or `paths-scoped` rule from.
	Body string `json:"body"`
}

// sortStickyRules orders a sticky set by rule name so regeneration produces no
// spurious diff and so the runtime's reverse-name-order drop is well defined.
func sortStickyRules(rules []StickyRule) []StickyRule {
	sorted := make([]StickyRule, len(rules))
	copy(sorted, rules)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	return sorted
}

// filterStickyRules contains the untrusted sticky array of a repo-supplied
// manifest before any of it reaches the filesystem or the injected context.
//
// Every field is attacker-chosen in a cloned repository. A Name outside
// ruleNamePattern is dropped and returned separately, so the caller reports it
// fail-closed per rule exactly like a containment violation, and a duplicate
// Name is dropped so one manifest cannot repeat a body to exhaust the aggregate
// cap. The kept set is name-ordered, which is what makes "drop whole rules in
// reverse rule-name order" a deterministic rule rather than manifest order.
//
// @AX:WARN [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: the trust boundary for the sticky set — every field arriving here is attacker-chosen in a cloned repository.
// @AX:REASON: ruleNamePattern, the duplicate check, and the maxManifestRules bound are the only filters between a hostile manifest and both the filesystem read and the injected model context; each one relaxed here reopens a different channel.
func filterStickyRules(rules []StickyRule) (kept []StickyRule, rejected []string) {
	if len(rules) > maxManifestRules {
		return nil, nil
	}

	seen := make(map[string]bool, len(rules))
	kept = make([]StickyRule, 0, len(rules))
	for _, rule := range rules {
		if !ruleNamePattern.MatchString(rule.Name) {
			rejected = append(rejected, sanitizeName(rule.Name))
			continue
		}
		if seen[rule.Name] {
			continue
		}
		seen[rule.Name] = true
		kept = append(kept, rule)
	}
	return sortStickyRules(kept), rejected
}

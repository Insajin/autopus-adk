package content

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ActivationResult holds the outcome of a single skill match evaluation.
type ActivationResult struct {
	// Skill is the matched skill definition.
	Skill SkillDefinition
	// Score is the normalized match confidence in the range [0.0, 1.0].
	Score float64
	// Reason describes why the skill was activated (e.g., "keyword: debug").
	Reason string
	// Source identifies the activation mechanism: "keyword", "regex", "context", or "intent".
	Source string
}

// SkillActivator evaluates an ActivationContext against the skill registry
// and produces an ordered list of ActivationResult values.
type SkillActivator struct {
	registry        *SkillRegistry
	autoActivate    bool
	maxActive       int
	categoryWeights map[string]int
}

// NewSkillActivator creates a SkillActivator backed by the given registry and config values.
func NewSkillActivator(
	registry *SkillRegistry,
	autoActivate bool,
	maxActive int,
	weights map[string]int,
) *SkillActivator {
	return &SkillActivator{
		registry:        registry,
		autoActivate:    autoActivate,
		maxActive:       maxActive,
		categoryWeights: weights,
	}
}

// isRegexTrigger reports whether the trigger string should be interpreted as a
// regular expression rather than a plain keyword. Triggers that start with "^"
// or contain "(", ".*", or ".+" are treated as regex patterns.
func isRegexTrigger(trigger string) bool {
	return strings.HasPrefix(trigger, "^") ||
		strings.Contains(trigger, "(") ||
		strings.Contains(trigger, ".*") ||
		strings.Contains(trigger, ".+")
}

// matchSkill evaluates a single skill against the query and returns the best
// ActivationResult for that skill. ok is false when no trigger matched.
func matchSkill(skill SkillDefinition, query string) (ActivationResult, bool) {
	lower := strings.ToLower(query)

	for _, trigger := range skill.Triggers {
		if isRegexTrigger(trigger) {
			re, err := regexp.Compile(trigger)
			if err != nil {
				// Skip malformed patterns gracefully.
				continue
			}
			if re.MatchString(query) {
				return ActivationResult{
					Skill:  skill,
					Score:  0.9,
					Reason: fmt.Sprintf("regex: %s", trigger),
					Source: "regex",
				}, true
			}
		} else {
			if strings.Contains(lower, strings.ToLower(trigger)) {
				return ActivationResult{
					Skill:  skill,
					Score:  0.8,
					Reason: fmt.Sprintf("keyword: %s", trigger),
					Source: "keyword",
				}, true
			}
		}
	}

	return ActivationResult{}, false
}

// Match evaluates all registered skills against ctx and returns every skill
// whose trigger matched, sorted by score descending.
// Returns nil when autoActivate is false.
func (a *SkillActivator) Match(ctx ActivationContext) []ActivationResult {
	if !a.autoActivate {
		return nil
	}

	var results []ActivationResult
	for _, skill := range a.registry.List() {
		if result, ok := matchSkill(skill, ctx.UserQuery); ok {
			results = append(results, result)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// Resolve applies category weights, re-sorts by adjusted score, and trims the
// list to at most maxActive entries.
func (a *SkillActivator) Resolve(results []ActivationResult) []ActivationResult {
	// Apply category weight multipliers.
	adjusted := make([]ActivationResult, len(results))
	for i, r := range results {
		if w, ok := a.categoryWeights[r.Skill.Category]; ok {
			r.Score = r.Score * (1 + float64(w)/100.0)
		}
		adjusted[i] = r
	}

	sort.Slice(adjusted, func(i, j int) bool {
		return adjusted[i].Score > adjusted[j].Score
	})

	if a.maxActive > 0 && len(adjusted) > a.maxActive {
		adjusted = adjusted[:a.maxActive]
	}

	return adjusted
}

// ActivateSkills runs the full activation pipeline: match → resolve → format notice.
// It returns the activation results and a human-readable notice string.
// When autoActivate is disabled, Match returns nil and both return values are zero.
func ActivateSkills(activator *SkillActivator, ctx ActivationContext) ([]ActivationResult, string) {
	matches := activator.Match(ctx)
	if matches == nil {
		return nil, ""
	}
	resolved := activator.Resolve(matches)
	notice := FormatActivationNotice(resolved)
	return resolved, notice
}

// FormatActivationNotice formats an activation notice string for the given results.
// Returns an empty string when results is empty.
func FormatActivationNotice(results []ActivationResult) string {
	if len(results) == 0 {
		return ""
	}

	parts := make([]string, len(results))
	for i, r := range results {
		parts[i] = fmt.Sprintf("%s (%s)", r.Skill.Name, r.Reason)
	}

	return "스킬 활성화: " + strings.Join(parts, ", ")
}

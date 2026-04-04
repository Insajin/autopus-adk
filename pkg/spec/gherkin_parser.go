package spec

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Gherkin scenario header: "### S1: ..." or "### Scenario: ..."
	reScenarioHeader = regexp.MustCompile(`^###\s+(?:S\d+:\s*|Scenario:\s*)(.+)`)

	// Gherkin step keywords
	reGherkinStep = regexp.MustCompile(`(?i)^\s*(Given|When|Then|And|But)\s+(.+)`)

	// Priority tag: "Priority: Must/Should/Nice"
	rePriority = regexp.MustCompile(`(?i)^\s*Priority:\s*(Must|Should|Nice)`)
)

// ParseGherkin parses acceptance criteria text in Gherkin format.
// Returns parsed criteria and any warnings encountered.
func ParseGherkin(text string) ([]Criterion, []string) {
	lines := strings.Split(text, "\n")

	var criteria []Criterion
	var warnings []string
	var current *scenarioBuilder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for scenario header
		if m := reScenarioHeader.FindStringSubmatch(trimmed); m != nil {
			if current != nil {
				criteria = append(criteria, current.build())
			}
			current = &scenarioBuilder{description: m[1]}
			continue
		}

		if current == nil {
			continue
		}

		// Check for priority tag
		if m := rePriority.FindStringSubmatch(trimmed); m != nil {
			current.priority = normalizeKeyword(m[1])
			continue
		}

		// Check for Gherkin step
		if m := reGherkinStep.FindStringSubmatch(trimmed); m != nil {
			current.steps = append(current.steps, GherkinStep{
				Keyword: normalizeKeyword(m[1]),
				Text:    strings.TrimSpace(m[2]),
			})
		}
	}

	// Flush last scenario
	if current != nil {
		criteria = append(criteria, current.build())
	}

	if len(criteria) == 0 {
		warnings = append(warnings, "no Gherkin scenarios found")
		return nil, warnings
	}

	// Assign auto IDs where missing
	assignAutoIDs(criteria)

	return criteria, warnings
}

// scenarioBuilder accumulates state for a single scenario during parsing.
type scenarioBuilder struct {
	description string
	priority    string
	steps       []GherkinStep
}

func (b *scenarioBuilder) build() Criterion {
	p := b.priority
	if p == "" {
		p = "Must"
	}
	return Criterion{
		Description: b.description,
		Priority:    p,
		Steps:       b.steps,
	}
}

// normalizeKeyword capitalizes the first letter of a Gherkin keyword.
func normalizeKeyword(kw string) string {
	if len(kw) == 0 {
		return kw
	}
	return strings.ToUpper(kw[:1]) + strings.ToLower(kw[1:])
}

// assignAutoIDs assigns AC-001, AC-002, ... to criteria without an ID.
func assignAutoIDs(criteria []Criterion) {
	counter := 1
	for i := range criteria {
		if criteria[i].ID == "" {
			criteria[i].ID = fmt.Sprintf("AC-%03d", counter)
			counter++
		}
	}
}

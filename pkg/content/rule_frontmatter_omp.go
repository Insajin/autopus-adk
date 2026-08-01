package content

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// ompDescriptionMaxRunes caps a synthesized description. omp renders the whole
// <domain-rules> list into every session prompt, so a long entry costs context
// on every turn without improving routing.
const ompDescriptionMaxRunes = 120

// ompRecognizedKeys are the frontmatter keys the omp rule parser reads. Every
// other key is dropped.
var ompRecognizedKeys = map[string]bool{
	"description":   true,
	"globs":         true,
	"alwaysApply":   true,
	"condition":     true,
	"astCondition":  true,
	"scope":         true,
	"interruptMode": true,
}

// TransformRuleForOMP transforms rule markdown content for the OMP platform.
// It parses the frontmatter, filters it to only recognized OMP keys, and checks for a non-empty body.
//
// A source that yields no recognized key gets a description synthesized from
// its body. Measured against omp 17.1.8: a rule reaches the session only when
// its frontmatter carries a description, so emitting a bare body writes the
// file to disk where no session ever discovers it. Synthesis never runs when
// any recognized key survives, which keeps trigger-carrying rules routed to the
// TTSR engine exactly as before — adding a description to one of those would
// move it into the always-listed block instead.
func TransformRuleForOMP(raw string) (string, error) {
	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		return "", fmt.Errorf("split frontmatter: %w", err)
	}

	body = strings.TrimSpace(body)
	if body == "" {
		return "", errors.New("rule body is empty")
	}

	ompMap, err := recognizedOMPFrontmatter(fm)
	if err != nil {
		return "", err
	}

	if len(ompMap) == 0 {
		if description := synthesizeOMPRuleDescription(body); description != "" {
			ompMap = map[string]interface{}{"description": description}
		}
	}
	if len(ompMap) == 0 {
		return body, nil
	}

	outFm, err := yaml.Marshal(ompMap)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(outFm)
	sb.WriteString("---\n\n")
	sb.WriteString(body)

	return sb.String(), nil
}

func recognizedOMPFrontmatter(fm string) (map[string]interface{}, error) {
	if fm == "" {
		return nil, nil
	}

	var rawMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(fm), &rawMap); err != nil {
		return nil, fmt.Errorf("unmarshal frontmatter: %w", err)
	}

	ompMap := make(map[string]interface{})
	for k, v := range rawMap {
		if ompRecognizedKeys[k] {
			ompMap[k] = v
		}
	}
	return ompMap, nil
}

// synthesizeOMPRuleDescription derives a one-line description from a rule body.
// omp renders each entry as "name (globs): description", so the document title
// is the most faithful summary; a body with no title falls back to its first
// prose line. The result is deterministic for a given body.
func synthesizeOMPRuleDescription(body string) string {
	title, prose := scanOMPDescriptionSources(body)
	if title != "" {
		return capOMPDescription(title)
	}
	return capOMPDescription(prose)
}

// scanOMPDescriptionSources walks the body once and returns the first H1 title
// and the first prose line. Fenced code is skipped so a shell comment inside a
// block is never mistaken for a heading.
func scanOMPDescriptionSources(body string) (title, prose string) {
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			if title == "" {
				title = sanitizeOMPDescription(strings.TrimPrefix(trimmed, "# "))
			}
			if title != "" {
				return title, prose
			}
			continue
		}
		if prose == "" && isOMPProseLine(trimmed) {
			prose = sanitizeOMPDescription(trimmed)
		}
	}
	return title, prose
}

// isOMPProseLine reports whether a line reads as a sentence rather than as
// markdown structure (heading, list item, table row, quote, or directive).
func isOMPProseLine(trimmed string) bool {
	switch trimmed[0] {
	case '#', '-', '*', '+', '|', '>', '<', '[':
		return false
	}
	if len(trimmed) > 1 && unicode.IsDigit(rune(trimmed[0])) && (trimmed[1] == '.' || trimmed[1] == ')') {
		return false
	}
	return true
}

var ompDescriptionLinkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// sanitizeOMPDescription flattens markdown inline markup and whitespace so the
// result is a single safe YAML scalar line.
func sanitizeOMPDescription(text string) string {
	text = ompDescriptionLinkRe.ReplaceAllString(text, "$1")
	text = strings.NewReplacer("`", "", "*", "", " ", " ").Replace(text)

	var sb strings.Builder
	lastSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !lastSpace {
				sb.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		sb.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(sb.String())
}

// capOMPDescription truncates on a rune boundary, preferring the last word
// boundary inside the cap so the result never splits a word mid-way.
func capOMPDescription(text string) string {
	runes := []rune(text)
	if len(runes) <= ompDescriptionMaxRunes {
		return text
	}
	clipped := string(runes[:ompDescriptionMaxRunes])
	if idx := strings.LastIndex(clipped, " "); idx > 0 {
		clipped = clipped[:idx]
	}
	return strings.TrimSpace(clipped)
}

package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const staticContractProvider = "spec-static"

var (
	reAllowlistBullet = regexp.MustCompile("(?m)^\\s*[-*]\\s+`([A-Za-z_][A-Za-z0-9_]*)`\\s*(?:[—:-]\\s*)?(.*)$")
	reBacktickIdent   = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*(?:\\.[A-Za-z_][A-Za-z0-9_]*)?)(?:=[^`]*)?`")
	reEventKindValue  = regexp.MustCompile("event_kind\\s*=\\s*\"?([A-Za-z_][A-Za-z0-9_]*)\"?")
	reObjectKey       = regexp.MustCompile("(?:^|[,\\s{])([A-Za-z_][A-Za-z0-9_]*)\\s*:")
	reFindingID       = regexp.MustCompile("^F-(\\d+)$")
)

// RunSpecContractAnalysis finds deterministic SPEC contract drift that LLM
// reviewers otherwise tend to rediscover across separate review iterations.
func RunSpecContractAnalysis(specDir string) ([]ReviewFinding, error) {
	docs, err := readSpecContractDocs(specDir)
	if err != nil {
		return nil, err
	}
	fullText := strings.Join(docs, "\n")
	if !strings.Contains(fullText, "Retained-Field Allowlist") {
		return nil, nil
	}

	allowlistText := sectionAfterHeading(fullText, "Retained-Field Allowlist")
	if allowlistText == "" {
		return nil, nil
	}
	allowed, containers := parseAllowlistFields(allowlistText)
	if len(allowed) == 0 {
		return nil, nil
	}

	tableText := sectionAfterHeading(fullText, "Per-event-kind")
	tableEvents, tableColumns := parseEventKindTable(tableText)
	referencedEvents, referencedColumns := parseContractReferences(fullText, containers)

	missingEvents := sortedMissing(referencedEvents, tableEvents)
	missingColumns := missingReferencedColumns(referencedColumns, allowed, containers)
	missingTableColumns := missingReferencedColumns(referencedColumns, tableColumns, containers)
	if len(missingEvents) == 0 && len(missingColumns) == 0 && len(missingTableColumns) == 0 {
		return nil, nil
	}

	parts := make([]string, 0, 3)
	if len(missingEvents) > 0 {
		parts = append(parts, "missing_event_kind_values: "+strings.Join(missingEvents, ", "))
	}
	if len(missingColumns) > 0 {
		parts = append(parts, "missing_allowlist_columns: "+strings.Join(missingColumns, ", "))
	}
	if len(missingTableColumns) > 0 {
		parts = append(parts, "missing_event_kind_table_columns: "+strings.Join(missingTableColumns, ", "))
	}

	return []ReviewFinding{{
		Provider:    staticContractProvider,
		Severity:    "major",
		Category:    FindingCategoryCompleteness,
		ScopeRef:    "spec-contract:retained-field-allowlist",
		Description: "Retained-field allowlist / per-event-kind table drift detected (" + strings.Join(parts, "; ") + "). Add every referenced event_kind and persisted column to the allowlist, per-event-kind table, and acceptance or rewrite the observable to use an already allowlisted container.",
		Status:      FindingStatusOpen,
	}}, nil
}

func readSpecContractDocs(specDir string) ([]string, error) {
	names := []string{"spec.md", "plan.md", "acceptance.md", "research.md"}
	docs := make([]string, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		docs = append(docs, string(data))
	}
	return docs, nil
}

func sectionAfterHeading(text, heading string) string {
	idx := strings.Index(text, heading)
	if idx < 0 {
		return ""
	}
	rest := text[idx:]
	next := regexp.MustCompile("(?m)^#{2,3}\\s+").FindAllStringIndex(rest, -1)
	for _, loc := range next {
		if loc[0] > 0 {
			return rest[:loc[0]]
		}
	}
	return rest
}

func parseAllowlistFields(section string) (map[string]struct{}, map[string]struct{}) {
	allowed := map[string]struct{}{}
	containers := map[string]struct{}{}
	for _, match := range reAllowlistBullet.FindAllStringSubmatch(section, -1) {
		field := match[1]
		allowed[field] = struct{}{}
		desc := strings.ToLower(match[2])
		if strings.Contains(desc, "object") || strings.Contains(desc, "shape") || strings.HasSuffix(field, "_meta") {
			containers[field] = struct{}{}
		}
	}
	return allowed, containers
}

func parseEventKindTable(section string) (map[string]struct{}, map[string]struct{}) {
	events := map[string]struct{}{}
	columns := map[string]struct{}{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") || strings.Contains(line, "---") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		if event := firstBacktickIdent(cells[1]); event != "" && event != "event_kind" {
			events[event] = struct{}{}
		}
		for _, field := range backtickIdents(cells[2]) {
			if !isEventValue(field, events) {
				columns[field] = struct{}{}
			}
		}
	}
	return events, columns
}

func parseContractReferences(text string, containers map[string]struct{}) (map[string]struct{}, map[string]struct{}) {
	events := map[string]struct{}{}
	columns := map[string]struct{}{}
	for _, line := range strings.Split(text, "\n") {
		if !contractReferenceLine(line) {
			continue
		}
		for _, match := range reEventKindValue.FindAllStringSubmatch(line, -1) {
			events[match[1]] = struct{}{}
		}
		for _, token := range backtickIdents(line) {
			if strings.HasPrefix(token, "event_kind") || token == "event_kind" {
				continue
			}
			addColumnReference(columns, token, containers, line)
		}
		for _, obj := range objectBodies(line) {
			for _, match := range reObjectKey.FindAllStringSubmatch(obj, -1) {
				addColumnReference(columns, match[1], containers, line)
			}
		}
	}
	return events, columns
}

func contractReferenceLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "event_kind") ||
		strings.Contains(lower, "row") ||
		strings.Contains(lower, "column") ||
		strings.Contains(line, "컬럼") ||
		strings.Contains(lower, "allowlist") ||
		strings.Contains(lower, "audit") ||
		strings.Contains(lower, "append")
}

func addColumnReference(out map[string]struct{}, token string, containers map[string]struct{}, line string) {
	token = strings.TrimSpace(token)
	if token == "" || isIgnorableContractToken(token) {
		return
	}
	if strings.Contains(token, ".") {
		parent := strings.SplitN(token, ".", 2)[0]
		if _, ok := containers[parent]; ok {
			out[parent] = struct{}{}
			return
		}
	}
	for container := range containers {
		if strings.Contains(line, container) {
			return
		}
	}
	out[token] = struct{}{}
}

func isIgnorableContractToken(token string) bool {
	switch token {
	case "true", "false", "null", "none", "event", "event_kind":
		return true
	}
	return strings.Contains(token, "/") || strings.Contains(token, "-")
}

func objectBodies(line string) []string {
	var out []string
	for start := strings.Index(line, "{"); start >= 0; {
		end := strings.Index(line[start:], "}")
		if end < 0 {
			break
		}
		out = append(out, line[start:start+end+1])
		next := start + end + 1
		if next >= len(line) {
			break
		}
		rel := strings.Index(line[next:], "{")
		if rel < 0 {
			break
		}
		start = next + rel
	}
	return out
}

func backtickIdents(s string) []string {
	matches := reBacktickIdent.FindAllStringSubmatch(s, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out
}

func firstBacktickIdent(s string) string {
	idents := backtickIdents(s)
	if len(idents) == 0 {
		return ""
	}
	return idents[0]
}

func isEventValue(token string, events map[string]struct{}) bool {
	_, ok := events[token]
	return ok
}

func sortedMissing(referenced, declared map[string]struct{}) []string {
	var missing []string
	for value := range referenced {
		if _, ok := declared[value]; !ok {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingReferencedColumns(referenced, declared, containers map[string]struct{}) []string {
	var missing []string
	for column := range referenced {
		if _, ok := declared[column]; ok {
			continue
		}
		if _, ok := containers[column]; ok {
			continue
		}
		missing = append(missing, column)
	}
	sort.Strings(missing)
	return missing
}

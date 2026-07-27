package e2e

import (
	"sort"
	"strings"
)

// @AX:ANCHOR [AUTO]: Preserve these executable-admission reason codes as a stable diagnostic wire contract.
// @AX:REASON: CLI JSON output and migration tooling consume the exact values to quarantine invalid scenarios deterministically.
// @AX:SPEC: SPEC-SCENARIO-PARSER-MIGRATION-001
const (
	ScenarioMissingStatus     = "scenario_missing_status"
	ScenarioInvalidStatus     = "scenario_invalid_status"
	ScenarioMissingCommand    = "scenario_missing_command"
	ScenarioMissingVerify     = "scenario_missing_verify"
	ScenarioUnsupportedVerify = "scenario_unsupported_verify"
	ScenarioDuplicateRef      = "scenario_duplicate_ref"
	ScenarioDuplicateID       = "scenario_duplicate_id"
	ScenarioDuplicateField    = "scenario_duplicate_field"
	ScenarioUnknownField      = "scenario_unknown_field"
	ScenarioMalformedHeader   = "scenario_malformed_header"
)

var executableScenarioFields = map[string]bool{
	"Command": true, "Precondition": true, "Env": true, "Expect": true,
	"Verify": true, "Depends": true, "Requires": true, "Status": true,
}

// ScenarioValidationIssue is the stable executable-admission diagnostic wire.
type ScenarioValidationIssue struct {
	Code        string `json:"code"`
	ScenarioRef string `json:"scenario_ref"`
	Field       string `json:"field"`
	Line        int    `json:"line"`
}

// ScenarioValidationReport keeps lenient parsing separate from executable
// admission. Runnable contains only explicit, valid active scenarios.
type ScenarioValidationReport struct {
	ScenarioSet  *ScenarioSet
	Runnable     []Scenario
	Issues       []ScenarioValidationIssue
	invalidCount int
}

// InvalidCount returns the number of quarantined scenario entries.
func (r *ScenarioValidationReport) InvalidCount() int {
	if r == nil {
		return 0
	}
	return r.invalidCount
}

type scenarioFieldOccurrence struct {
	value string
	line  int
}

type scenarioValidationRecord struct {
	ref        string
	id         string
	headerLine int
	fields     map[string][]scenarioFieldOccurrence
	rawInvalid bool
}

// ValidateScenarios parses leniently, then produces deterministic executable
// admission diagnostics from the raw document so duplicate and line evidence
// is not lost.
// @AX:WARN [AUTO]: This admission policy has more than eight malformed, duplicate, status, command, and verification branches.
// @AX:REASON: Missing one branch can reintroduce false-green execution or cross-scenario field bleed before build and shell dispatch.
func ValidateScenarios(content []byte) (*ScenarioValidationReport, error) {
	set, err := ParseScenarios(content)
	if err != nil {
		return nil, err
	}
	records, scanIssues, malformedCount := scanScenarioRecords(content)
	report := &ScenarioValidationReport{
		ScenarioSet: set,
		Runnable:    []Scenario{},
		Issues:      append([]ScenarioValidationIssue(nil), scanIssues...),
	}
	invalid := make([]bool, len(records))
	refOwner := make(map[string]int, len(records))
	idOwner := make(map[string]int, len(records))

	for i, record := range records {
		invalid[i] = record.rawInvalid
		if previous, exists := refOwner[record.ref]; exists {
			invalid[previous], invalid[i] = true, true
			report.addIssue(ScenarioDuplicateRef, record.ref, "Ref", record.headerLine)
		} else {
			refOwner[record.ref] = i
		}
		if previous, exists := idOwner[record.id]; exists {
			invalid[previous], invalid[i] = true, true
			report.addIssue(ScenarioDuplicateID, record.ref, "ID", record.headerLine)
		} else {
			idOwner[record.id] = i
		}
		for field, occurrences := range record.fields {
			if len(occurrences) > 1 {
				invalid[i] = true
				report.addIssue(ScenarioDuplicateField, record.ref, field, occurrences[1].line)
			}
		}

		status, statusLine, statusPresent := record.lastField("Status")
		switch {
		case !statusPresent || strings.TrimSpace(status) == "":
			invalid[i] = true
			report.addIssue(ScenarioMissingStatus, record.ref, "Status", record.headerLine)
		case !isScenarioStatus(status):
			invalid[i] = true
			report.addIssue(ScenarioInvalidStatus, record.ref, "Status", statusLine)
		}

		requiresExecutableFields := status == "active" || !statusPresent || strings.TrimSpace(status) == ""
		command, commandLine, commandPresent := record.lastField("Command")
		if requiresExecutableFields && (!commandPresent || strings.Trim(strings.TrimSpace(command), "`") == "") {
			invalid[i] = true
			report.addIssue(ScenarioMissingCommand, record.ref, "Command", fallbackLine(commandLine, record.headerLine))
		}
		verify, verifyLine, verifyPresent := record.lastField("Verify")
		primitives := SplitVerifyPrimitives(verify)
		if requiresExecutableFields && (!verifyPresent || len(primitives) == 0) {
			invalid[i] = true
			report.addIssue(ScenarioMissingVerify, record.ref, "Verify", fallbackLine(verifyLine, record.headerLine))
		}
		for _, primitive := range primitives {
			if _, parseErr := ParseVerifyPrimitive(primitive); parseErr != nil {
				invalid[i] = true
				report.addIssue(ScenarioUnsupportedVerify, record.ref, "Verify", verifyLine)
				break
			}
		}
	}

	for i, scenario := range set.Scenarios {
		if i < len(invalid) && !invalid[i] && scenario.Status == "active" {
			report.Runnable = append(report.Runnable, scenario)
		}
	}
	report.invalidCount = malformedCount
	for _, isInvalid := range invalid {
		if isInvalid {
			report.invalidCount++
		}
	}
	sort.Slice(report.Issues, func(i, j int) bool {
		left, right := report.Issues[i], report.Issues[j]
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.ScenarioRef != right.ScenarioRef {
			return left.ScenarioRef < right.ScenarioRef
		}
		return left.Field < right.Field
	})
	return report, nil
}

func scanScenarioRecords(content []byte) ([]scenarioValidationRecord, []ScenarioValidationIssue, int) {
	var records []scenarioValidationRecord
	var issues []ScenarioValidationIssue
	current := -1
	malformed := 0
	for index, line := range strings.Split(string(content), "\n") {
		lineNumber := index + 1
		if match := reScenHeader.FindStringSubmatch(line); match != nil {
			records = append(records, scenarioValidationRecord{
				ref: match[1], id: strings.TrimSpace(match[2]), headerLine: lineNumber,
				fields: make(map[string][]scenarioFieldOccurrence),
			})
			current = len(records) - 1
			continue
		}
		if strings.HasPrefix(line, "### ") {
			ref := malformedScenarioRef(line)
			issues = append(issues, ScenarioValidationIssue{
				Code: ScenarioMalformedHeader, ScenarioRef: ref, Field: "Header", Line: lineNumber,
			})
			malformed++
			current = -1
			continue
		}
		if current < 0 {
			continue
		}
		if match := reField.FindStringSubmatch(line); match != nil {
			field := match[1]
			if !executableScenarioFields[field] {
				records[current].rawInvalid = true
				issues = append(issues, ScenarioValidationIssue{
					Code: ScenarioUnknownField, ScenarioRef: records[current].ref,
					Field: field, Line: lineNumber,
				})
				continue
			}
			records[current].fields[field] = append(records[current].fields[field], scenarioFieldOccurrence{
				value: match[2], line: lineNumber,
			})
		}
	}
	return records, issues, malformed
}

func (r *ScenarioValidationReport) addIssue(code, ref, field string, line int) {
	r.Issues = append(r.Issues, ScenarioValidationIssue{
		Code: code, ScenarioRef: ref, Field: field, Line: line,
	})
}

func (r scenarioValidationRecord) lastField(name string) (string, int, bool) {
	values := r.fields[name]
	if len(values) == 0 {
		return "", 0, false
	}
	last := values[len(values)-1]
	return last.value, last.line, true
}

func isScenarioStatus(status string) bool {
	switch status {
	case "active", "deprecated", "skip", "reference":
		return true
	default:
		return false
	}
}

func malformedScenarioRef(line string) string {
	value := strings.TrimSpace(strings.TrimPrefix(line, "### "))
	if index := strings.IndexByte(value, ':'); index >= 0 {
		value = value[:index]
	}
	if fields := strings.Fields(value); len(fields) > 0 {
		return fields[0]
	}
	return "S?"
}

func fallbackLine(line, fallback int) int {
	if line > 0 {
		return line
	}
	return fallback
}

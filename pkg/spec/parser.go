package spec

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// EARS (Easy Approach to Requirements Syntax) 패턴 정규식
	// 출처: Mavin, Wilkinson, Harwood, Novak (2009)
	// "Easy Approach to Requirements Syntax (EARS)" — IEEE International Requirements Engineering Conference
	// Optional은 WHEN+IF+THEN 복합 패턴 (EventDriven보다 먼저 확인)
	reOptional    = regexp.MustCompile(`(?i)WHEN\s+.+\s+IF\s+.+\s+THEN\s+.+`)
	reEventDriven = regexp.MustCompile(`(?i)WHEN\s+.+\s+THEN\s+.+`)
	reStateDriven = regexp.MustCompile(`(?i)WHERE\s+.+\s+THEN\s+.+`)
	reUnwanted    = regexp.MustCompile(`(?i)IF\s+.+\s+THEN\s+.+`)
	// Ubiquitous: 한국어 (시스템은/시스템이/시스템) + 영어 패턴 모두 지원
	reUbiquitous = regexp.MustCompile(`(?i)(시스템[은이]?|system|The system)\s+SHALL\s+.+`)
	// reUppercaseShall matches the bare uppercase EARS modal verb so that a
	// requirement-looking line that fails detectEARSType is surfaced as a
	// warning rather than silently skipped (SPEC-SPECREV-002 REQ-004). Only the
	// uppercase token triggers it to avoid prose false positives.
	reUppercaseShall = regexp.MustCompile(`\bSHALL\b`)
)

// ParseEARS는 텍스트에서 EARS 패턴 요구사항을 파싱한다.
// 후방호환 wrapper: 경고는 버리고 요구사항만 반환한다.
func ParseEARS(text string) ([]Requirement, error) {
	reqs, _, err := ParseEARSWithWarnings(text)
	return reqs, err
}

// ParseEARSWithWarnings parses EARS requirements and additionally returns a
// warning for every non-bullet, non-comment line that carries an uppercase
// SHALL token but matches no EARS type. This makes the previously silent
// detectEARSType skip observable (SPEC-SPECREV-002 REQ-004).
func ParseEARSWithWarnings(text string) ([]Requirement, []string, error) {
	var reqs []Requirement
	var warnings []string
	counter := 1

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}

		reqType := detectEARSType(line)
		if reqType == "" {
			if reUppercaseShall.MatchString(line) {
				warnings = append(warnings, fmt.Sprintf("인식되지 않은 SHALL 라인: %s", line))
			}
			continue
		}

		reqs = append(reqs, Requirement{
			ID:          fmt.Sprintf("REQ-%03d", counter),
			Type:        reqType,
			Description: line,
		})
		counter++
	}

	return reqs, warnings, nil
}

// detectEARSType는 문장에서 EARS 패턴 유형을 감지한다.
func detectEARSType(line string) EARSType {
	// Optional은 EventDriven보다 먼저 확인 (더 구체적)
	if reOptional.MatchString(line) {
		return EARSOptional
	}
	if reEventDriven.MatchString(line) {
		return EARSEventDriven
	}
	if reStateDriven.MatchString(line) {
		return EARSStateDriven
	}
	if reUnwanted.MatchString(line) {
		return EARSUnwanted
	}
	if reUbiquitous.MatchString(line) {
		return EARSUbiquitous
	}
	return ""
}

package orchestra

import (
	"fmt"
	"strings"
	"unicode"
)

// MergeConsensus는 여러 응답에서 합의점을 추출한다.
// threshold는 합의 기준 비율 (0.0 ~ 1.0)이다.
// 반환값: (병합된 결과, 요약 문자열)
func MergeConsensus(responses []ProviderResponse, threshold float64) (string, string) {
	if len(responses) == 0 {
		return "", "응답 없음"
	}
	if merged, summary, ok := mergeReviewerFindingConsensus(responses, threshold); ok {
		return merged, summary
	}

	// Try structured parsing first
	if merged, summary := MergeStructuredConsensus(responses, threshold); merged != "" {
		return merged, summary
	}

	// 각 응답을 줄 단위로 분리
	linesByProvider := make([][]string, len(responses))
	for i, r := range responses {
		linesByProvider[i] = splitLines(r.Output)
	}

	// 모든 고유 줄 수집 (정규화 기준)
	seen := make(map[string]bool)
	var allNormLines []string
	normToOrig := make(map[string]string)

	for _, lines := range linesByProvider {
		for _, line := range lines {
			norm := normalizeLine(line)
			if norm == "" {
				continue
			}
			if !seen[norm] {
				seen[norm] = true
				allNormLines = append(allNormLines, norm)
				normToOrig[norm] = line
			}
		}
	}

	total := len(responses)
	var agreedLines []string
	var disputedLines []string

	agreedCount := 0
	for _, norm := range allNormLines {
		count := 0
		for _, lines := range linesByProvider {
			for _, line := range lines {
				if normalizeLine(line) == norm {
					count++
					break
				}
			}
		}

		ratio := float64(count) / float64(total)
		if ratio >= threshold {
			agreedLines = append(agreedLines, fmt.Sprintf("✓ %s", normToOrig[norm]))
			agreedCount++
		} else {
			// 동의하지 않은 줄: 어느 프로바이더에서 나왔는지 표시
			var providers []string
			for i, lines := range linesByProvider {
				for _, line := range lines {
					if normalizeLine(line) == norm {
						providers = append(providers, responses[i].Provider)
						break
					}
				}
			}
			disputedLines = append(disputedLines, fmt.Sprintf("△ %s [%s]", normToOrig[norm], strings.Join(providers, ", ")))
		}
	}

	var sb strings.Builder
	if len(agreedLines) > 0 {
		sb.WriteString("## 합의된 내용\n")
		sb.WriteString(strings.Join(agreedLines, "\n"))
		sb.WriteString("\n")
	}
	if len(disputedLines) > 0 {
		sb.WriteString("\n## 이견이 있는 내용\n")
		sb.WriteString(strings.Join(disputedLines, "\n"))
	}

	summary := fmt.Sprintf("합의율: %d/%d (%.0f%%)", agreedCount, len(allNormLines), float64(agreedCount)/float64(max1(len(allNormLines)))*100)
	return sb.String(), summary
}

// FormatPipeline은 파이프라인 결과를 단계별로 포맷한다.
func FormatPipeline(responses []ProviderResponse) string {
	var sb strings.Builder
	for i, r := range responses {
		sb.WriteString(fmt.Sprintf("## Stage %d: (by %s)\n", i+1, r.Provider))
		sb.WriteString(r.Output)
		sb.WriteString("\n")
	}
	return sb.String()
}

// FormatDebate는 토론 결과를 포맷한다.
func FormatDebate(responses []ProviderResponse) string {
	if len(responses) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 토론 결과\n\n")

	for _, r := range responses {
		sb.WriteString(fmt.Sprintf("### %s의 의견\n", r.Provider))
		sb.WriteString(cleanScreenOutput(r.Output)) // R3: clean noise before display
		sb.WriteString("\n\n")
	}

	// R3: findDifferences handles its own cleaning internally
	if len(responses) >= 2 {
		sb.WriteString("## 주요 차이점\n")
		diffs := findDifferences(responses)
		if len(diffs) == 0 {
			sb.WriteString("(주요 차이점 없음)\n")
		} else {
			for _, d := range diffs {
				sb.WriteString(fmt.Sprintf("- %s\n", d))
			}
		}
	}

	return sb.String()
}

// findDifferences identifies key differences between responses.
// Inputs should be pre-cleaned via cleanScreenOutput for accurate comparison;
// this function also applies cleaning internally to handle direct callers.
func findDifferences(responses []ProviderResponse) []string {
	if len(responses) < 2 {
		return nil
	}

	// Clean outputs to strip ANSI, MCP noise, and prompt lines before comparison
	cleaned := make([]ProviderResponse, len(responses))
	copy(cleaned, responses)
	for i := range cleaned {
		cleaned[i].Output = cleanScreenOutput(cleaned[i].Output)
	}

	baseLines := make(map[string]bool)
	for _, line := range splitLines(cleaned[0].Output) {
		norm := normalizeLine(line)
		if norm != "" {
			baseLines[norm] = true
		}
	}

	var diffs []string
	for i := 1; i < len(cleaned); i++ {
		for _, line := range splitLines(cleaned[i].Output) {
			norm := normalizeLine(line)
			if norm != "" && !baseLines[norm] {
				diffs = append(diffs, fmt.Sprintf("%s에서만: %s", cleaned[i].Provider, line))
			}
		}
	}
	return diffs
}

// normalizeLine은 비교를 위해 줄을 정규화한다.
// 앞뒤 공백 제거, 소문자 변환, 특수문자 제거
func normalizeLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	// 소문자로 변환하고 문장부호 등 제거
	var sb strings.Builder
	for _, r := range strings.ToLower(line) {
		if !unicode.IsPunct(r) && r != ' ' || r == ' ' {
			sb.WriteRune(r)
		}
	}
	return strings.TrimSpace(sb.String())
}

// splitLines는 텍스트를 비어있지 않은 줄 목록으로 분리한다.
func splitLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// max1은 0으로 나누기를 방지하기 위한 최솟값 1을 반환한다.
func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

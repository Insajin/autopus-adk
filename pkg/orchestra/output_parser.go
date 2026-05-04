package orchestra

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OutputParser parses provider JSON responses into typed Go structs.
type OutputParser struct{}

// ParseDebaterR1 parses a debater Round 1 JSON response.
func (op *OutputParser) ParseDebaterR1(raw string) (*DebaterR1Output, error) {
	var out DebaterR1Output
	if err := op.unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse debater_r1: %w", err)
	}
	if len(out.Ideas) == 0 {
		return nil, fmt.Errorf("parse debater_r1: at least 1 idea required")
	}
	return &out, nil
}

// ParseDebaterR2 parses a debater Round 2 JSON response.
func (op *OutputParser) ParseDebaterR2(raw string) (*DebaterR2Output, error) {
	var out DebaterR2Output
	if err := op.unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse debater_r2: %w", err)
	}
	return &out, nil
}

// ParseJudge parses a judge synthesis JSON response.
func (op *OutputParser) ParseJudge(raw string) (*JudgeOutput, error) {
	var out JudgeOutput
	if err := op.unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse judge: %w", err)
	}
	if out.Recommendation == "" {
		return nil, fmt.Errorf("parse judge: recommendation required")
	}
	return &out, nil
}

// ParseReviewer parses a reviewer verdict JSON response.
func (op *OutputParser) ParseReviewer(raw string) (*ReviewerOutput, error) {
	var out ReviewerOutput
	if err := op.unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse reviewer: %w", err)
	}
	switch out.Verdict {
	case "PASS", "REVISE", "REJECT":
		// valid
	default:
		return nil, fmt.Errorf("parse reviewer: invalid verdict %q (expected PASS, REVISE, or REJECT)", out.Verdict)
	}
	if strings.TrimSpace(out.Summary) == "" {
		return nil, fmt.Errorf("parse reviewer: summary required")
	}
	for _, item := range out.Checklist {
		switch item.Status {
		case "PASS", "FAIL", "N/A":
			// valid; N/A allowed for items that do not apply (e.g. doc-only SPEC security checks).
		default:
			return nil, fmt.Errorf("parse reviewer: invalid checklist status %q", item.Status)
		}
	}
	for _, item := range out.FindingStatus {
		switch item.Status {
		case "open", "resolved", "regressed":
			// valid
		default:
			return nil, fmt.Errorf("parse reviewer: invalid finding status %q", item.Status)
		}
	}
	return &out, nil
}

// ParseAny parses raw text as JSON of the given role type.
func (op *OutputParser) ParseAny(raw string, role string) (any, error) {
	switch role {
	case "debater_r1":
		return op.ParseDebaterR1(raw)
	case "debater_r2":
		return op.ParseDebaterR2(raw)
	case "judge":
		return op.ParseJudge(raw)
	case "reviewer":
		return op.ParseReviewer(raw)
	default:
		return nil, fmt.Errorf("unknown role: %q", role)
	}
}

// unmarshal extracts JSON from raw text and unmarshals it.
func (op *OutputParser) unmarshal(raw string, target any) error {
	extracted := extractJSON(raw)
	if extracted == "" {
		return fmt.Errorf("no JSON found in response")
	}
	return json.Unmarshal([]byte(extracted), target)
}

// extractJSON finds and returns JSON content from raw provider output.
// Handles: plain JSON, markdown code blocks, Claude stream-json envelope.
func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Try direct parse first.
	if json.Valid([]byte(raw)) {
		return tryUnwrapClaudeEnvelope(raw)
	}

	// Strip markdown code blocks.
	if stripped := stripMarkdownJSON(raw); stripped != "" && json.Valid([]byte(stripped)) {
		return tryUnwrapClaudeEnvelope(stripped)
	}

	// Find first { and matching last }.
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		candidate := raw[start : end+1]
		if json.Valid([]byte(candidate)) {
			return tryUnwrapClaudeEnvelope(candidate)
		}
	}

	return ""
}

// stripMarkdownJSON removes ```json ... ``` wrapping.
func stripMarkdownJSON(raw string) string {
	idx := strings.Index(raw, "```json")
	if idx < 0 {
		idx = strings.Index(raw, "```")
		if idx < 0 {
			return ""
		}
	}
	// Find the newline after the opening fence.
	start := strings.Index(raw[idx:], "\n")
	if start < 0 {
		return ""
	}
	start += idx + 1

	// Find the closing fence.
	end := strings.LastIndex(raw, "```")
	if end <= start {
		return ""
	}
	return strings.TrimSpace(raw[start:end])
}

// tryUnwrapClaudeEnvelope checks for Claude's stream-json envelope format
// and extracts the inner content text if found.
func tryUnwrapClaudeEnvelope(raw string) string {
	var envelope struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return raw
	}
	if envelope.Type == "result" && len(envelope.Content) > 0 {
		for _, c := range envelope.Content {
			if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
				inner := strings.TrimSpace(c.Text)
				if json.Valid([]byte(inner)) {
					return inner
				}
			}
		}
	}
	return raw
}

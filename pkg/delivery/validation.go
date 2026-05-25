package delivery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

var generatedRuntimeDenyPatterns = []string{
	".autopus/*-manifest.json",
	".autopus/context/signatures.md",
	".autopus/plugins/**",
	".autopus/brainstorms/**",
	".autopus/orchestra/**",
	".autopus/design/imports/**",
	".autopus/canary/**",
	".autopus/qa/**",
	".autopus/runtime/**",
	".claude/**",
	".codex/**",
	".gemini/**",
	".opencode/**",
	".agents/plugins/marketplace.json",
	"config.toml",
}

type ValidationError struct {
	Problems []string `json:"problems"`
}

func (e *ValidationError) Error() string {
	return "delivery validation failed: " + strings.Join(e.Problems, "; ")
}

func ValidatePhase(phase Phase) error {
	for _, candidate := range CanonicalPhases() {
		if phase == candidate {
			return nil
		}
	}
	return fmt.Errorf("unknown phase %q", phase)
}

func ValidateProviderMode(mode ProviderMode) error {
	switch mode {
	case ProviderClaudeSubscriptionInteractive, ProviderClaudeAPIHeadless, ProviderCodexSubscriptionInteractive, ProviderCodexAPIHeadless:
		return nil
	default:
		return fmt.Errorf("unknown provider mode %q", mode)
	}
}

func ValidateStatus(status PhaseStatus) error {
	switch status {
	case StatusPassed, StatusWarn, StatusFailed, StatusBlocked, StatusApprovalRequired, StatusRetryRequested:
		return nil
	default:
		return fmt.Errorf("invalid status %q", status)
	}
}

func ValidateRedactionStatus(status RedactionStatus) error {
	switch status {
	case RedactionClean, RedactionRedacted, RedactionBlocked:
		return nil
	default:
		return fmt.Errorf("invalid redaction_status %q", status)
	}
}

func ParsePhaseResultEnvelope(data []byte) (PhaseResultEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope PhaseResultEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return envelope, fmt.Errorf("decode phase result envelope: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return envelope, fmt.Errorf("decode phase result envelope: trailing JSON value")
	} else if err != io.EOF {
		return envelope, fmt.Errorf("decode phase result envelope: trailing JSON value: %w", err)
	}
	return envelope, nil
}

func ValidatePhaseResultEnvelope(envelope PhaseResultEnvelope) error {
	var problems []string
	if envelope.SchemaVersion != PhaseResultSchemaV1 {
		problems = append(problems, fmt.Sprintf("schema_version must be %q", PhaseResultSchemaV1))
	}
	if strings.TrimSpace(envelope.RequestID) == "" {
		problems = append(problems, "request_id is required")
	}
	if err := ValidatePhase(envelope.Phase); err != nil {
		problems = append(problems, err.Error())
	}
	if err := ValidateStatus(envelope.Status); err != nil {
		problems = append(problems, err.Error())
	}
	if strings.TrimSpace(envelope.Summary) == "" {
		problems = append(problems, "summary is required")
	}
	if envelope.ChangedFiles == nil {
		problems = append(problems, "changed_files is required")
	}
	if strings.TrimSpace(envelope.TestStatus) == "" {
		problems = append(problems, "test_status is required")
	}
	if envelope.EvidenceRefs == nil {
		problems = append(problems, "evidence_refs is required")
	}
	if envelope.Blockers == nil {
		problems = append(problems, "blockers is required")
	}
	if strings.TrimSpace(envelope.NextRequiredAction) == "" {
		problems = append(problems, "next_required_action is required")
	}
	if err := ValidateRedactionStatus(envelope.RedactionStatus); err != nil {
		problems = append(problems, err.Error())
	}
	if envelope.RedactionStatus == RedactionBlocked &&
		(envelope.Status == StatusPassed || envelope.Status == StatusWarn) {
		problems = append(problems, "redaction_status blocked cannot be used with passed or warn status")
	}
	if drift := GeneratedRuntimeDrift(envelope.ChangedFiles); len(drift) > 0 {
		problems = append(problems, "generated/runtime changed-file drift: "+strings.Join(drift, ", "))
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

func ValidatePhaseResultJSON(data []byte) (PhaseResultEnvelope, error) {
	envelope, err := ParsePhaseResultEnvelope(data)
	if err != nil {
		return envelope, err
	}
	return envelope, ValidatePhaseResultEnvelope(envelope)
}

func GeneratedRuntimeDenyPatterns() []string {
	patterns := append([]string(nil), generatedRuntimeDenyPatterns...)
	sort.Strings(patterns)
	return patterns
}

func GeneratedRuntimeDrift(files []string) []string {
	var drift []string
	for _, file := range files {
		normalized := normalizeChangedPath(file)
		if normalized == "" {
			continue
		}
		if IsGeneratedRuntimePath(normalized) {
			drift = append(drift, normalized)
		}
	}
	sort.Strings(drift)
	return drift
}

func ValidateNoGeneratedRuntimeDrift(files []string) error {
	drift := GeneratedRuntimeDrift(files)
	if len(drift) == 0 {
		return nil
	}
	return fmt.Errorf("generated/runtime changed-file drift: %s", strings.Join(drift, ", "))
}

func IsGeneratedRuntimePath(file string) bool {
	normalized := normalizeChangedPath(file)
	for _, pattern := range generatedRuntimeDenyPatterns {
		if denyPatternMatches(pattern, normalized) {
			return true
		}
	}
	return false
}

func denyPatternMatches(pattern, file string) bool {
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**") + "/"
		return strings.HasPrefix(file, prefix)
	}
	matched, err := path.Match(pattern, file)
	return err == nil && matched
}

func normalizeChangedPath(file string) string {
	normalized := filepath.ToSlash(strings.TrimSpace(file))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = path.Clean(normalized)
	if normalized == "." {
		return ""
	}
	return normalized
}

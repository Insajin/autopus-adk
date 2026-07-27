package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// @AX:ANCHOR [AUTO]: Preserve this exact allowlist as the executable verification grammar.
// @AX:REASON: Parser, validator, runner, and CLI admission all depend on unsupported primitives failing closed.
// @AX:SPEC: SPEC-SCENARIO-PARSER-MIGRATION-001
const (
	VerifyExitCode       = "exit_code"
	VerifyStdoutContains = "stdout_contains"
	VerifyStderrEmpty    = "stderr_empty"
	VerifyFileExists     = "file_exists"
	VerifyFileContains   = "file_contains"
)

var exitCodePrimitiveRE = regexp.MustCompile(`^exit_code\((-?\d+)\)$`)

// VerifyPrimitive is the parsed representation of the supported executable
// verification grammar.
type VerifyPrimitive struct {
	Kind      string
	ExitCode  int
	Arguments []string
}

// ParseVerifyPrimitive accepts only the documented verification grammar.
func ParseVerifyPrimitive(raw string) (VerifyPrimitive, error) {
	primitive := strings.TrimSpace(raw)
	if match := exitCodePrimitiveRE.FindStringSubmatch(primitive); match != nil {
		code, err := strconv.Atoi(match[1])
		if err != nil {
			return VerifyPrimitive{}, fmt.Errorf("invalid exit code: %w", err)
		}
		return VerifyPrimitive{Kind: VerifyExitCode, ExitCode: code}, nil
	}
	if primitive == "stderr_empty()" {
		return VerifyPrimitive{Kind: VerifyStderrEmpty}, nil
	}
	for _, candidate := range []struct {
		name  string
		arity int
	}{
		{name: VerifyStdoutContains, arity: 1},
		{name: VerifyFileExists, arity: 1},
		{name: VerifyFileContains, arity: 2},
	} {
		prefix := candidate.name + "("
		if !strings.HasPrefix(primitive, prefix) || !strings.HasSuffix(primitive, ")") {
			continue
		}
		args, err := decodePrimitiveArguments(primitive[len(prefix) : len(primitive)-1])
		if err != nil || len(args) != candidate.arity {
			return VerifyPrimitive{}, fmt.Errorf("%s requires %d JSON string argument(s)", candidate.name, candidate.arity)
		}
		if candidate.name == VerifyFileExists || candidate.name == VerifyFileContains {
			if err := validateRelativeArtifactPath(args[0]); err != nil {
				return VerifyPrimitive{}, err
			}
		}
		return VerifyPrimitive{Kind: candidate.name, Arguments: args}, nil
	}
	return VerifyPrimitive{}, fmt.Errorf("unsupported verification primitive %q", primitive)
}

func decodePrimitiveArguments(raw string) ([]string, error) {
	var args []string
	decoder := json.NewDecoder(strings.NewReader("[" + raw + "]"))
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON data")
		}
		return nil, err
	}
	return args, nil
}

func validateRelativeArtifactPath(path string) error {
	if path == "" || strings.TrimSpace(path) != path || strings.ContainsRune(path, '\x00') ||
		strings.Contains(path, `\`) || filepath.IsAbs(path) || filepath.VolumeName(path) != "" ||
		hasWindowsArtifactDrivePrefix(path) {
		return fmt.Errorf("artifact path must be a clean project-relative path")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != path {
		return fmt.Errorf("artifact path must be a clean project-relative path")
	}
	return nil
}

func hasWindowsArtifactDrivePrefix(path string) bool {
	return len(path) >= 2 &&
		((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) &&
		path[1] == ':'
}

// EvaluateVerifyPrimitive executes one supported primitive and fails closed
// for malformed or unsupported input.
func EvaluateVerifyPrimitive(raw string, result *RunnerResult) VerifyResult {
	primitive := strings.TrimSpace(raw)
	if result == nil {
		return VerifyResult{Primitive: primitive, Pass: false, Message: "runner result is required"}
	}
	parsed, err := ParseVerifyPrimitive(primitive)
	if err != nil {
		return VerifyResult{Primitive: primitive, Pass: false, Message: err.Error()}
	}

	var evaluated VerifyResult
	switch parsed.Kind {
	case VerifyExitCode:
		evaluated = CheckExitCode(parsed.ExitCode, result.ExitCode)
	case VerifyStdoutContains:
		evaluated = CheckStdoutContains(parsed.Arguments[0], result.Stdout)
	case VerifyStderrEmpty:
		evaluated = CheckStderrEmpty(result.Stderr)
	case VerifyFileExists:
		evaluated = CheckFileExists(filepath.Join(result.WorkDir, filepath.FromSlash(parsed.Arguments[0])))
	case VerifyFileContains:
		evaluated = CheckFileContains(
			filepath.Join(result.WorkDir, filepath.FromSlash(parsed.Arguments[0])),
			parsed.Arguments[1],
		)
	default:
		evaluated = VerifyResult{Pass: false, Message: "unsupported verification primitive"}
	}
	evaluated.Primitive = primitive
	return evaluated
}

// SplitVerifyPrimitives splits a comma-separated Verify field without
// separating commas inside JSON strings or function argument lists.
func SplitVerifyPrimitives(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var result []string
	start, depth := 0, 0
	quoted, escaped := false, false
	for i, r := range raw {
		switch {
		case escaped:
			escaped = false
		case quoted && r == '\\':
			escaped = true
		case r == '"':
			quoted = !quoted
		case !quoted && r == '(':
			depth++
		case !quoted && r == ')' && depth > 0:
			depth--
		case !quoted && depth == 0 && r == ',':
			if item := strings.TrimSpace(raw[start:i]); item != "" {
				result = append(result, item)
			}
			start = i + 1
		}
	}
	if item := strings.TrimSpace(raw[start:]); item != "" {
		result = append(result, item)
	}
	return result
}

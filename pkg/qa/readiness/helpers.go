package readiness

import (
	"path/filepath"
	"strings"
)

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func stringList(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if text := stringValue(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func listOfMaps(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if doc, ok := item.(map[string]any); ok {
			out = append(out, doc)
		}
	}
	return out
}

func boolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: redaction aliases accept legacy clean/redacted evidence states as safe.
func redactionPassed(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == "passed" || typed == "redacted" || typed == "clean"
	case map[string]any:
		return redactionPassed(typed["status"])
	default:
		return false
	}
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: ownership matching is fail-closed unless workspace/repo ids and normalized repo roots agree.
func ownershipMatches(input Input, value any) bool {
	doc, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if stringValue(doc["workspace_id"]) != input.WorkspaceID || stringValue(doc["repo_id"]) != input.RepoID {
		return false
	}
	wantRoot := filepath.ToSlash(filepath.Clean(input.RepoRoot))
	gotRoot := filepath.ToSlash(filepath.Clean(filepath.Join(input.WorkspaceRoot, stringValue(doc["repo_root"]))))
	return wantRoot == gotRoot || strings.HasSuffix(wantRoot, filepath.ToSlash(stringValue(doc["repo_root"])))
}

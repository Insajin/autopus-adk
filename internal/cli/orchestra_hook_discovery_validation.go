package cli

import (
	"os"
	"path/filepath"
	"strings"
)

type hookDecision uint8

const (
	hookUnspecified hookDecision = iota
	hookDisabled
	hookActive
)

type hookCapabilityDecision struct {
	completion    hookDecision
	startup       hookDecision
	disableAllSet bool
	disableAll    bool
}

func applyHookDecision(target *bool, decision hookDecision) {
	switch decision {
	case hookDisabled:
		*target = false
	case hookActive:
		*target = true
	}
}

func disabledCandidateEvents(candidate hookModeCandidate) hookCapabilityDecision {
	decision := hookCapabilityDecision{}
	if candidate.completionEvent != "" {
		decision.completion = hookDisabled
	}
	if candidate.startupEvent != "" {
		decision.startup = hookDisabled
	}
	return decision
}

func hooksDocumentDisabled(document map[string]any) bool {
	return hookScopeDisabled(document)
}

func hookScopeDisabled(scope map[string]any) bool {
	enabled, exists := scope["enabled"].(bool)
	return exists && !enabled
}

func hookEventDecision(
	events map[string]any,
	event, root, expectedAsset string,
	honorEnabled bool,
) hookDecision {
	if event == "" {
		return hookUnspecified
	}
	value, exists := events[event]
	if !exists {
		// An absent event never overrides a broader scope. Provider-specific
		// scalar/container disable decisions are resolved separately.
		return hookUnspecified
	}
	return inspectHookCommands(value, root, expectedAsset, honorEnabled)
}

func inspectHookCommands(value any, root, expectedAsset string, honorEnabled bool) hookDecision {
	switch value := value.(type) {
	case []any:
		decision := hookUnspecified
		for _, item := range value {
			decision = combineHookDecisions(
				decision, inspectHookCommands(item, root, expectedAsset, honorEnabled),
			)
		}
		return decision
	case map[string]any:
		if honorEnabled && hookScopeDisabled(value) {
			if containsAutopusHookCommand(value) {
				return hookDisabled
			}
			return hookUnspecified
		}
		decision := hookUnspecified
		for key, item := range value {
			if strings.EqualFold(key, "command") {
				if command, ok := item.(string); ok {
					decision = combineHookDecisions(
						decision, hookCommandDecision(command, root, expectedAsset),
					)
				}
			}
			decision = combineHookDecisions(
				decision, inspectHookCommands(item, root, expectedAsset, honorEnabled),
			)
		}
		return decision
	}
	return hookUnspecified
}

func combineHookDecisions(left, right hookDecision) hookDecision {
	if left == hookActive || right == hookActive {
		return hookActive
	}
	if left == hookDisabled || right == hookDisabled {
		return hookDisabled
	}
	return hookUnspecified
}

func hookCommandDecision(command, root, expectedAsset string) hookDecision {
	if isSemanticAutopusHookCommand(command) {
		// An explicit `autopus hook ...` command delegates asset ownership to the
		// installed binary and therefore has no project script to stat here.
		return hookActive
	}
	if !isManagedHookScriptCommand(command) {
		return hookUnspecified
	}
	if !commandReferencesExpectedAsset(command, expectedAsset) ||
		!managedHookAssetExecutable(root, expectedAsset) {
		return hookDisabled
	}
	return hookActive
}

func isSemanticAutopusHookCommand(command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 2 {
		return false
	}
	executable := strings.Trim(fields[0], `"'`)
	return binaryIdentity(executable) == "autopus" && strings.EqualFold(fields[1], "hook")
}

func isManagedHookScriptCommand(command string) bool {
	command = strings.ToLower(strings.ReplaceAll(command, `\`, "/"))
	return strings.Contains(command, "/hooks/autopus/")
}

func commandReferencesExpectedAsset(command, expectedAsset string) bool {
	command = filepath.ToSlash(strings.ReplaceAll(command, `\`, "/"))
	expectedAsset = filepath.ToSlash(filepath.Clean(expectedAsset))
	return expectedAsset != "" && strings.Contains(command, expectedAsset)
}

func managedHookAssetExecutable(root, expectedAsset string) bool {
	if root == "" || expectedAsset == "" || filepath.IsAbs(expectedAsset) || !filepath.IsLocal(expectedAsset) {
		return false
	}
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(expectedAsset)))
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

func containsAutopusHookCommand(value any) bool {
	switch value := value.(type) {
	case []any:
		for _, item := range value {
			if containsAutopusHookCommand(item) {
				return true
			}
		}
	case map[string]any:
		for key, item := range value {
			if strings.EqualFold(key, "command") {
				if command, ok := item.(string); ok &&
					(isSemanticAutopusHookCommand(command) || isManagedHookScriptCommand(command)) {
					return true
				}
			}
			if containsAutopusHookCommand(item) {
				return true
			}
		}
	}
	return false
}

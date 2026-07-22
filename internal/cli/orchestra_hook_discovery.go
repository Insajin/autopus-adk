package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// @AX:NOTE [AUTO]: hook discovery merges user, project, and cwd wiring by precedence.
func isHookModeAvailable() bool {
	return discoverHookCapabilities().anyCompletion()
}

type hookCapability struct {
	completion bool
	startup    bool
}

type hookDiscovery map[string]hookCapability

func newHookDiscovery() hookDiscovery {
	return hookDiscovery{
		"claude":      {},
		"codex":       {},
		"antigravity": {},
		"gemini":      {},
		"opencode":    {},
	}
}

func (d hookDiscovery) capability(provider string) hookCapability {
	return d[hookRuntimeFamily(provider, "")]
}

func (d hookDiscovery) capabilityFor(name, binary string) hookCapability {
	return d[hookRuntimeFamily(name, binary)]
}

func (d hookDiscovery) anyCompletion() bool {
	for _, capability := range d {
		if capability.completion {
			return true
		}
	}
	return false
}

func discoverHookCapabilities() hookDiscovery {
	candidates := make([]hookModeCandidate, 0, 12)
	runtimeRoot := effectiveHookRuntimeRoot()
	for _, root := range hookDiscoveryRoots() {
		candidates = append(candidates, hookModeCandidates(root, runtimeRoot)...)
	}
	return discoverHookCapabilitiesInCandidates(candidates...)
}

func hookDiscoveryRoots() []string {
	roots := make([]string, 0, 3)
	seen := make(map[string]bool, 3)
	add := func(root string) {
		root = filepath.Clean(root)
		if root == "." || seen[root] {
			return
		}
		seen[root] = true
		roots = append(roots, root)
	}

	if home, err := os.UserHomeDir(); err == nil {
		add(home)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return roots
	}
	if projectRoot := nearestAutopusRoot(cwd); projectRoot != "" {
		add(projectRoot)
	}
	add(cwd)
	return roots
}

func effectiveHookRuntimeRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if root := nearestAutopusRoot(cwd); root != "" {
		return root
	}
	return filepath.Clean(cwd)
}

func nearestAutopusRoot(cwd string) string {
	for dir := filepath.Clean(cwd); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "autopus.yaml")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return ""
		}
	}
}

type hookModeCandidate struct {
	path            string
	configRoot      string // settings ownership; fallback when no execution root is available
	runtimeRoot     string // effective project/cwd root used by generated hook commands
	provider        string
	container       string
	completionEvent string
	startupEvent    string
	completionAsset string
	startupAsset    string
}

func hookModeCandidates(root, runtimeRoot string) []hookModeCandidate {
	if root == "" {
		return nil
	}
	candidates := []hookModeCandidate{
		{
			configRoot: root, runtimeRoot: runtimeRoot,
			path: filepath.Join(root, ".claude", "settings.json"), provider: "claude",
			container: "hooks", completionEvent: "Stop", startupEvent: "SessionStart",
			completionAsset: ".claude/hooks/autopus/hook-claude-stop.sh",
			startupAsset:    ".claude/hooks/autopus/hook-claude-sessionstart.sh",
		},
	}
	if isProjectHookRoot(root, runtimeRoot) {
		candidates = append(candidates, hookModeCandidate{
			configRoot: root, runtimeRoot: runtimeRoot,
			path: filepath.Join(root, ".claude", "settings.local.json"), provider: "claude",
			container: "hooks", completionEvent: "Stop", startupEvent: "SessionStart",
			completionAsset: ".claude/hooks/autopus/hook-claude-stop.sh",
			startupAsset:    ".claude/hooks/autopus/hook-claude-sessionstart.sh",
		})
	}
	return append(candidates,
		hookModeCandidate{
			configRoot: root, runtimeRoot: runtimeRoot,
			path: filepath.Join(root, ".codex", "hooks.json"), provider: "codex",
			container: "hooks", completionEvent: "Stop", startupEvent: "SessionStart",
			completionAsset: ".codex/hooks/autopus/hook-codex-stop.sh",
			startupAsset:    ".codex/hooks/autopus/hook-codex-sessionstart.sh",
		},
		hookModeCandidate{
			configRoot: root, runtimeRoot: runtimeRoot,
			path: filepath.Join(root, ".agents", "hooks.json"), provider: "antigravity",
			container: "autopus", completionEvent: "Stop",
			completionAsset: ".gemini/hooks/autopus/hook-gemini-stop.sh",
		},
		hookModeCandidate{
			configRoot: root, runtimeRoot: runtimeRoot,
			path: filepath.Join(root, ".gemini", "settings.json"), provider: "gemini",
			container: "hooks", completionEvent: "AfterAgent",
			completionAsset: ".gemini/hooks/autopus/hook-gemini-afteragent.sh",
		},
	)
}

func isProjectHookRoot(root, runtimeRoot string) bool {
	home, err := os.UserHomeDir()
	return err != nil || filepath.Clean(root) != filepath.Clean(home) ||
		filepath.Clean(root) == filepath.Clean(runtimeRoot)
}

func discoverHookCapabilitiesInCandidates(candidates ...hookModeCandidate) hookDiscovery {
	discovery := newHookDiscovery()
	claudeHooks := hookCapability{}
	claudeDisabled := false
	// Candidates are ordered from broadest to nearest scope. Claude hook arrays
	// accumulate while its disableAllHooks scalar follows nearest-value wins;
	// other provider schemas retain explicit active/disabled precedence.
	for _, candidate := range candidates {
		decision := readHookCapabilityDecision(candidate)
		provider := hookRuntimeFamily(candidate.provider, "")
		if provider == "claude" {
			if decision.completion == hookActive {
				claudeHooks.completion = true
			}
			if decision.startup == hookActive {
				claudeHooks.startup = true
			}
			if decision.disableAllSet {
				claudeDisabled = decision.disableAll
			}
			continue
		}
		current := discovery[provider]
		applyHookDecision(&current.completion, decision.completion)
		applyHookDecision(&current.startup, decision.startup)
		discovery[provider] = current
	}
	if claudeDisabled {
		claudeHooks = hookCapability{}
	}
	discovery["claude"] = claudeHooks
	return discovery
}

func readHookCapabilityDecision(candidate hookModeCandidate) hookCapabilityDecision {
	if candidate.path == "" || candidate.provider == "" || candidate.container == "" {
		return hookCapabilityDecision{}
	}
	data, err := os.ReadFile(candidate.path)
	if err != nil {
		return hookCapabilityDecision{}
	}
	var document map[string]any
	if json.Unmarshal(data, &document) != nil {
		return hookCapabilityDecision{}
	}
	isClaude := hookRuntimeFamily(candidate.provider, "") == "claude"
	decision := hookCapabilityDecision{}
	if isClaude {
		if disabled, ok := document["disableAllHooks"].(bool); ok {
			decision.disableAllSet = true
			decision.disableAll = disabled
		}
	} else if hooksDocumentDisabled(document) {
		return disabledCandidateEvents(candidate)
	}
	events, ok := document[candidate.container].(map[string]any)
	if !ok {
		return decision
	}
	if !isClaude && hookScopeDisabled(events) {
		return disabledCandidateEvents(candidate)
	}
	assetRoot := hookCandidateAssetRoot(candidate)
	decision.completion = hookEventDecision(
		events, candidate.completionEvent, assetRoot, candidate.completionAsset, !isClaude,
	)
	decision.startup = hookEventDecision(
		events, candidate.startupEvent, assetRoot, candidate.startupAsset, !isClaude,
	)
	return decision
}

func hookCandidateAssetRoot(candidate hookModeCandidate) string {
	if hookRuntimeFamily(candidate.provider, "") == "antigravity" && candidate.configRoot != "" {
		return candidate.configRoot
	}
	if candidate.runtimeRoot != "" {
		return candidate.runtimeRoot
	}
	return candidate.configRoot
}

func hookModeAvailableInCandidates(candidates ...hookModeCandidate) bool {
	return discoverHookCapabilitiesInCandidates(candidates...).anyCompletion()
}

// hookModeAvailableInDirs checks injected Claude settings paths for Stop hooks.
// Callers provide fixed paths; user-supplied path segments are not accepted.
func hookModeAvailableInDirs(globalPath, projectPath string) bool {
	candidates := make([]hookModeCandidate, 0, 2)
	for _, path := range []string{globalPath, projectPath} {
		candidates = append(candidates, hookModeCandidate{
			path: path, runtimeRoot: effectiveHookRuntimeRoot(),
			provider: "claude", container: "hooks", completionEvent: "Stop",
		})
	}
	return hookModeAvailableInCandidates(candidates...)
}

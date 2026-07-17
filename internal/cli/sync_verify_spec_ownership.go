package cli

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

var codePrefixes = []string{"pkg/", "cmd/", "internal/", "src/", "app/"}
var ownedPathPattern = regexp.MustCompile(`^[A-Za-z0-9_./@+-]+\.(?:go|ts|tsx|js|jsx|py|rs|md|json|yaml|yml|toml)$`)
var inlineCodePathPattern = regexp.MustCompile("`([^`\\n]+)`")

func extractOwnedTokens(text string) []string {
	seen := map[string]bool{}
	for _, match := range inlineCodePathPattern.FindAllStringSubmatch(text, -1) {
		addOwnedToken(seen, match[1])
	}
	for _, field := range strings.Fields(text) {
		if strings.Contains(field, "`") {
			continue
		}
		addOwnedToken(seen, strings.Trim(field, "'\"()[]{}<>,;"))
	}
	var tokens []string
	for token := range seen {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

func addOwnedToken(seen map[string]bool, token string) {
	if !ownedPathPattern.MatchString(token) || strings.Contains(token, "\\") || path.IsAbs(token) {
		return
	}
	clean := path.Clean(token)
	if clean != token || clean == "." || strings.HasPrefix(clean, "../") {
		return
	}
	seen[token] = true
}

func referencedModules(text string, modules map[string]bool) []string {
	found := map[string]bool{}
	for _, token := range extractOwnedTokens(text) {
		for module := range modules {
			if !strings.HasPrefix(token, module+"/") {
				continue
			}
			rest := strings.TrimPrefix(token, module+"/")
			for _, prefix := range codePrefixes {
				if strings.HasPrefix(rest, prefix) {
					found[module] = true
				}
			}
		}
	}
	var owned []string
	for module := range found {
		owned = append(owned, module)
	}
	sort.Strings(owned)
	return owned
}

func classifySpecLocation(specID, repoPath string, owned []string) string {
	if len(owned) == 0 {
		return ""
	}
	if len(owned) == 1 {
		module := owned[0]
		switch {
		case repoPath == ".":
			return fmt.Sprintf(
				"WARN  misplacement: SPEC %s at root (.) references only module %s paths -> expected %s/.autopus/specs/",
				specID, diagnosticRepoLabel(module), displayPath(module))
		case repoPath != module:
			return fmt.Sprintf(
				"WARN  location-mismatch: SPEC %s at %s references only module %s paths -> expected %s/.autopus/specs/",
				specID, diagnosticRepoLabel(repoPath), diagnosticRepoLabel(module), displayPath(module))
		}
		return ""
	}
	if repoPath != "." {
		return fmt.Sprintf(
			"WARN  location-mismatch: SPEC %s at %s references cross-module paths -> detected owner cross-module, expected .autopus/specs/ (root)",
			specID, diagnosticRepoLabel(repoPath))
	}
	return ""
}

func detectSpecViolations(repos []repoDirty, modules map[string]bool) ([]string, error) {
	ids := map[string]bool{}
	for _, repo := range repos {
		for _, file := range repo.Files {
			if id, ok := specIDFromPath(file.Rel); ok {
				ids[id] = true
			}
		}
	}
	var ordered []string
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)

	var warnings []string
	for _, id := range ordered {
		host, err := locateSpecHost(repos, id)
		if err != nil {
			return nil, err
		}
		text, err := readSpecReferences(host, id)
		if err != nil {
			return nil, err
		}
		if warning := classifySpecLocation(id, host.repoPath, referencedModules(text, modules)); warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return warnings, nil
}

func splitSpecOwnership(
	repos []repoDirty,
	specID string,
	classified workspaceClassification,
) (owned, unrelated []string, err error) {
	host, err := locateSpecHost(repos, specID)
	if err != nil {
		return nil, nil, err
	}
	text, err := readSpecReferences(host, specID)
	if err != nil {
		return nil, nil, err
	}
	modules := moduleSet(repos)
	ownedSet := map[string]bool{}
	for _, token := range extractOwnedTokens(text) {
		ownedSet[canonicalWorkspaceToken(token, host.repoPath, modules)] = true
	}
	specPrefix := workspaceRel(host.repoPath, ".autopus/specs/"+specID+"/")
	eligible := eligibleWorkspacePaths(classified)
	all := inventoryWorkspacePaths(repos)
	for _, rel := range all {
		if eligible[rel] && (strings.HasPrefix(rel, specPrefix) || ownedSet[rel]) {
			owned = append(owned, rel)
		} else {
			unrelated = append(unrelated, rel)
		}
	}
	return owned, unrelated, nil
}

func canonicalWorkspaceToken(token, hostRepo string, modules map[string]bool) string {
	for module := range modules {
		if token == module || strings.HasPrefix(token, module+"/") {
			return token
		}
	}
	return workspaceRel(hostRepo, token)
}

func eligibleWorkspacePaths(classified workspaceClassification) map[string]bool {
	eligible := map[string]bool{}
	for _, group := range classified.PhaseA {
		for _, rel := range group.Files {
			eligible[workspaceRel(group.RepoPath, rel)] = true
		}
	}
	for _, rel := range classified.PhaseB.Files {
		eligible[rel] = true
	}
	return eligible
}

func inventoryWorkspacePaths(repos []repoDirty) []string {
	seen := map[string]bool{}
	for _, repo := range repos {
		for _, file := range repo.Files {
			seen[workspaceRel(repo.Path, file.Rel)] = true
		}
		for _, rel := range repo.TrackedIgnored {
			seen[workspaceRel(repo.Path, rel)] = true
		}
	}
	var paths []string
	for rel := range seen {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return paths
}

func filterPlanForOwned(classified workspaceClassification, owned []string) workspaceClassification {
	wanted := map[string]bool{}
	for _, rel := range owned {
		wanted[rel] = true
	}
	filtered := classified
	filtered.PhaseA = nil
	for _, group := range classified.PhaseA {
		selected := filterPhaseGroup(group, wanted)
		if len(selected.Files) > 0 {
			filtered.PhaseA = append(filtered.PhaseA, selected)
		}
	}
	filtered.PhaseB = filterPhaseGroup(classified.PhaseB, wanted)
	return filtered
}

func filterPhaseGroup(group phaseGroup, wanted map[string]bool) phaseGroup {
	selected := phaseGroup{RepoPath: group.RepoPath}
	selectPaths := func(paths []string) []string {
		var out []string
		for _, rel := range paths {
			if wanted[workspaceRel(group.RepoPath, rel)] {
				out = append(out, rel)
			}
		}
		return out
	}
	selected.Files = selectPaths(group.Files)
	selected.AddFiles = selectPaths(group.AddFiles)
	selected.UpdateFiles = selectPaths(group.UpdateFiles)
	selected.StagedOnly = selectPaths(group.StagedOnly)
	return selected
}

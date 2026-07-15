package promptlayer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func resolveConditionalContextProfiles(command string, profile CommandContextProfile, requested []ContextProfileName) ([]ContextProfileName, error) {
	selected := append([]ContextProfileName(nil), requested...)
	allowed := make(map[ContextProfileName]bool, len(profile.Conditional))
	for _, name := range profile.Conditional {
		allowed[name] = true
	}
	seen := make(map[ContextProfileName]bool, len(selected))
	resolved := make([]ContextProfileName, 0, len(selected))
	for _, name := range selected {
		if !allowed[name] {
			return nil, fmt.Errorf("conditional context profile %q is not declared for command %s", name, command)
		}
		if !seen[name] {
			seen[name] = true
			resolved = append(resolved, name)
		}
	}
	sort.Slice(resolved, func(i, j int) bool { return resolved[i] < resolved[j] })
	return resolved, nil
}

func availableDefaultConditionalDocuments(root, command string) ([]string, error) {
	if command != "go" && command != "review" {
		return nil, nil
	}
	var available []string
	for _, ref := range documentsForProfiles([]ContextProfileName{ProfileArchitecture}) {
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(ref)))
		switch {
		case err == nil:
			available = append(available, ref)
		case os.IsNotExist(err):
			continue
		default:
			return nil, fmt.Errorf("inspect conditional context %s: %w", ref, err)
		}
	}
	return available, nil
}

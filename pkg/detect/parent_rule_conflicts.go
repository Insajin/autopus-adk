package detect

import (
	"os"
	"path/filepath"
)

// ParentRuleConflict describes a rule namespace inherited from a parent directory.
type ParentRuleConflict struct {
	ParentDir string // Parent directory containing the conflict.
	RulesDir  string // Parent .claude/rules directory.
	Namespace string // Rule namespace, for example "moai".
}

// CheckParentRuleConflicts finds inherited .claude/rules namespaces above projectDir.
func CheckParentRuleConflicts(projectDir string) []ParentRuleConflict {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil
	}

	var conflicts []ParentRuleConflict
	current := filepath.Dir(absDir)

	for {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		rulesDir := filepath.Join(current, ".claude", "rules")
		entries, err := os.ReadDir(rulesDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					conflicts = append(conflicts, ParentRuleConflict{
						ParentDir: current,
						RulesDir:  rulesDir,
						Namespace: entry.Name(),
					})
				}
			}
		}
		current = parent
	}

	return conflicts
}

package constraint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultPath is the conventional location for project constraint definitions.
const DefaultPath = ".autopus/context/constraints.yaml"

// Registry manages project-level anti-pattern constraints loaded from YAML.
type Registry struct {
	constraints []Constraint
}

// Load reads constraints from the given YAML file path.
// Returns an empty Registry if the file does not exist (not treated as an error).
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, fmt.Errorf("read constraints file: %w", err)
	}

	var cf ConstraintsFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse constraints YAML: %w", err)
	}

	return &Registry{constraints: cf.Deny}, nil
}

// LoadFromDir loads constraints from the default path within a project directory.
func LoadFromDir(projectDir string) (*Registry, error) {
	return Load(filepath.Join(projectDir, DefaultPath))
}

// Constraints returns all loaded constraints.
func (r *Registry) Constraints() []Constraint {
	return r.constraints
}

// ByCategory returns constraints filtered by the given category.
func (r *Registry) ByCategory(cat Category) []Constraint {
	var result []Constraint
	for _, c := range r.constraints {
		if c.Category == cat {
			result = append(result, c)
		}
	}
	return result
}

// IsEmpty returns true when no constraints are loaded.
func (r *Registry) IsEmpty() bool {
	return len(r.constraints) == 0
}

// GeneratePromptText produces a "NEVER do:" section suitable for agent prompt injection.
// Returns an empty string when the registry has no constraints.
func (r *Registry) GeneratePromptText() string {
	if r.IsEmpty() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Anti-Pattern Constraints\n\n")
	sb.WriteString("NEVER do the following in this project:\n\n")

	for i, c := range r.constraints {
		fmt.Fprintf(&sb, "%d. **NEVER**: `%s`\n", i+1, c.Pattern)
		fmt.Fprintf(&sb, "   - Reason: %s\n", c.Reason)
		fmt.Fprintf(&sb, "   - Instead: %s\n", c.Suggest)
		fmt.Fprintf(&sb, "   - Category: %s\n", c.Category)
	}

	return sb.String()
}

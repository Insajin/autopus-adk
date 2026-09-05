package rulecond

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Fixed project-root-relative locations of the compiled conditional surface.
// REQ-CONDRULE-FIRE-11 forbids taking either location from an environment
// variable, a command flag, or the hook stdin payload, so both are constants.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: cross-cutting path contract shared by the compiler, the dispatcher, internal/cli/check_rules_hygiene.go drift mapping, and the parity classifier.
// @AX:REASON: Changing a value here without updating the drift source mapping and the parity `conditional/` case turns generated files into reported orphans.
const (
	// ManifestRelPath is the compiled conditional rule manifest.
	ManifestRelPath = ".claude/hooks/autopus/conditional-rules.json"
	// BodyRootRelPath is the conditional body root holding every relocated
	// hook-fired rule body.
	BodyRootRelPath = ".claude/hooks/autopus/conditional"
	// ClaudeRulesRelDir is the baseline rule directory Claude Code loads at
	// session start.
	ClaudeRulesRelDir = ".claude/rules/autopus"
)

// Byte caps fixed by REQ-CONDRULE-FIRE-05 and REQ-CONDRULE-FIRE-08.
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-CONDRULE-001: 4000/8000 are the context budget for injected rule text; the compile-time check and the dispatch-time check both read MaxBodyBytes, so they cannot drift apart.
const (
	// MaxBodyBytes caps a single rule body.
	MaxBodyBytes = 4000
	// MaxContextBytes caps the aggregate injected context.
	MaxContextBytes = 8000
)

// Injection format markers. Every injected body is preceded by a line built as
// RuleHeaderPrefix + rule name, and rules dropped by the aggregate cap are
// named on one line built as TruncationPrefix + names.
const (
	RuleHeaderPrefix = "## Rule: "
	TruncationPrefix = "> Omitted to fit the context budget: "
)

// Rule is one parsed rule source file: the trigger frontmatter of
// REQ-CONDRULE-SCHEMA-01 plus the rule body with its frontmatter removed.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: shared trigger schema — SPEC-STICKYRULE-001 consumes AlwaysApply and SPEC-OMP-001 depends on InterruptMode, AstCondition, and Raw surviving verbatim.
// @AX:REASON: Dropping a field or interpreting InterruptMode/AstCondition here would silently change rule meaning on the sibling tracks that re-emit this frontmatter.
type Rule struct {
	// Name is the `name` frontmatter value, defaulting to the file stem.
	Name string
	// Description is the `description` frontmatter value.
	Description string
	// Category is the `category` frontmatter value.
	Category string
	// Conditions holds the `condition` regexes. The field accepts a single
	// string or a string list; both shapes land here.
	Conditions []string
	// Scopes holds the `scope` entries, such as `tool:bash`.
	Scopes []string
	// Globs holds the `globs` file patterns.
	Globs []string
	// AlwaysApply is the `alwaysApply` flag consumed by SPEC-STICKYRULE-001.
	AlwaysApply bool
	// SkillScoped marks a rule that only applies inside the `/auto` workflow
	// skills that reference it. It carries no runtime trigger: the claude-code
	// compiler relocates the body out of baseline context and no dispatcher
	// entry is produced, so the referencing skill is what loads it.
	SkillScoped bool
	// InterruptMode is preserved verbatim and never interpreted here
	// (REQ-CONDRULE-SCHEMA-04).
	InterruptMode string
	// AstCondition is preserved verbatim and never evaluated here
	// (REQ-CONDRULE-SCHEMA-04).
	AstCondition string
	// Body is the rule text with the frontmatter block removed.
	Body string
	// SourceFile is the rule file name, used to name a rule in errors.
	SourceFile string
	// Frontmatter is the verbatim frontmatter block without its delimiters.
	Frontmatter string
	// Raw holds every frontmatter key as parsed, so unrecognized keys survive
	// downstream re-emission without being interpreted here.
	Raw map[string]any
}

// ruleFrontmatter is the typed view of a rule frontmatter block.
type ruleFrontmatter struct {
	Name          string     `yaml:"name"`
	Description   string     `yaml:"description"`
	Category      string     `yaml:"category"`
	Condition     stringList `yaml:"condition"`
	Scope         stringList `yaml:"scope"`
	Globs         stringList `yaml:"globs"`
	AlwaysApply   bool       `yaml:"alwaysApply"`
	SkillScoped   bool       `yaml:"skillScoped"`
	InterruptMode string     `yaml:"interruptMode"`
	AstCondition  string     `yaml:"astCondition"`
}

// stringList accepts either a single YAML scalar or a YAML sequence of scalars.
type stringList []string

// UnmarshalYAML decodes a scalar into a one-element list and a sequence into
// the list itself, so `condition: "x"` and `condition: ["x", "y"]` both parse.
func (s *stringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var single string
		if err := node.Decode(&single); err != nil {
			return err
		}
		if strings.TrimSpace(single) == "" {
			*s = nil
			return nil
		}
		*s = []string{single}
		return nil
	case yaml.SequenceNode:
		var many []string
		if err := node.Decode(&many); err != nil {
			return err
		}
		out := make([]string, 0, len(many))
		for _, item := range many {
			if strings.TrimSpace(item) == "" {
				continue
			}
			out = append(out, item)
		}
		*s = out
		return nil
	default:
		return fmt.Errorf("expected a string or a string list")
	}
}

// ParseRule parses one rule file. A file without a frontmatter block stays
// valid: its whole content becomes the body and its name falls back to the
// file stem, which is what keeps the untriggered rules parseable unchanged.
func ParseRule(filename string, raw []byte) (*Rule, error) {
	frontmatter, body := splitFrontmatter(string(raw))

	rule := &Rule{
		Body:        strings.TrimLeft(body, "\n"),
		SourceFile:  filename,
		Frontmatter: frontmatter,
		Raw:         map[string]any{},
	}

	if strings.TrimSpace(frontmatter) != "" {
		var typed ruleFrontmatter
		if err := yaml.Unmarshal([]byte(frontmatter), &typed); err != nil {
			return nil, fmt.Errorf("rule %s: frontmatter parse failed: %w", filename, err)
		}
		if err := yaml.Unmarshal([]byte(frontmatter), &rule.Raw); err != nil {
			return nil, fmt.Errorf("rule %s: frontmatter parse failed: %w", filename, err)
		}
		rule.Name = typed.Name
		rule.Description = typed.Description
		rule.Category = typed.Category
		rule.Conditions = typed.Condition
		rule.Scopes = typed.Scope
		rule.Globs = typed.Globs
		rule.AlwaysApply = typed.AlwaysApply
		rule.SkillScoped = typed.SkillScoped
		rule.InterruptMode = typed.InterruptMode
		rule.AstCondition = typed.AstCondition
	}

	if rule.Name == "" {
		rule.Name = strings.TrimSuffix(filename, ".md")
	}
	return rule, nil
}

// splitFrontmatter separates a leading `---` delimited frontmatter block from
// the body. It mirrors the unexported helper of the same name in
// pkg/content/skills.go, which is not importable from here.
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-CONDRULE-001: deliberate duplication of pkg/content/skills.go::splitFrontmatter — the two must keep splitting identically or a rule body diverges between the skill and rule pipelines.
func splitFrontmatter(content string) (frontmatter, body string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", content
	}
	frontmatter = rest[:idx]
	body = rest[idx+4:]
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}
	return frontmatter, body
}

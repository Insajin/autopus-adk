package claude

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// contentRuleDir is the embedded rule source directory.
const contentRuleDir = "rules"

// fileSizeLimitRuleFile is the one rule the claude adapter renders from
// templates/claude/rules/file-size-limit.md.tmpl instead of copying, because
// its exclusion list depends on the detected stack. It therefore never travels
// the compiled mapping path even though it classifies as paths-scoped.
const fileSizeLimitRuleFile = "file-size-limit.md"

// claudeConditionalSurface is the shared classification for prepared mappings
// and transactional generation, so every entry point agrees on rule placement.
type claudeConditionalSurface struct {
	// classes maps a rule source file name to its classification.
	classes map[string]rulecond.Class
	// mappings are the files the compiler owns: relocated hook-fired bodies,
	// the dispatcher manifest, and every paths-scoped rule the template path
	// does not already render.
	mappings []adapter.FileMapping
}

// relocates reports whether a rule source file is emitted by the compiler
// rather than by the verbatim copy path. An `always` rule stays on the copy
// path so its emitted bytes are unchanged (REQ-CONDRULE-SCHEMA-02).
func (s *claudeConditionalSurface) relocates(filename string) bool {
	if s == nil {
		return false
	}
	class, ok := s.classes[filename]
	return ok && class != rulecond.ClassAlways
}

// claudeConditionalRules parses every embedded rule, classifies it, and
// compiles the triggered ones.
//
// The dispatcher hook entries of the compilation are deliberately dropped here:
// pkg/content/hooks.go owns hook registration for every platform and derives
// its entries from this same rulecond compilation, so emitting them again would
// produce a duplicate settings.json entry. Because both sides now derive from
// one compilation, a rule scoped to a non-Bash tool registers its matcher
// automatically rather than needing a second hardcoded entry.
func claudeConditionalRules() (*claudeConditionalSurface, error) {
	rules, err := claudeRuleSources()
	if err != nil {
		return nil, err
	}
	compiled, err := rulecond.CompileClaude(rules)
	if err != nil {
		return nil, fmt.Errorf("조건부 룰 컴파일 실패: %w", err)
	}

	surface := &claudeConditionalSurface{classes: make(map[string]rulecond.Class, len(rules))}
	for _, rule := range rules {
		surface.classes[rule.SourceFile] = rulecond.Classify(rule)
	}

	for _, file := range compiled.RuleFiles {
		if filepath.Base(file.TargetPath) == fileSizeLimitRuleFile {
			continue
		}
		surface.mappings = append(surface.mappings, file)
	}
	surface.mappings = append(surface.mappings, compiled.Bodies...)
	surface.mappings = append(surface.mappings, compiled.Manifest)
	return surface, nil
}

// claudeRuleSurface compiles the conditional surface when the content
// directory being copied is the rule directory, and returns nil otherwise so
// skills, agents, and hooks keep their unconditional copy behaviour.
func claudeRuleSurface(subDir string) (*claudeConditionalSurface, error) {
	if subDir != contentRuleDir {
		return nil, nil
	}
	return claudeConditionalRules()
}

// claudeRuleSources parses every embedded rule source file.
func claudeRuleSources() ([]*rulecond.Rule, error) {
	entries, err := fs.ReadDir(contentfs.FS, contentRuleDir)
	if err != nil {
		return nil, fmt.Errorf("컨텐츠 디렉터리 읽기 실패 %s: %w", contentRuleDir, err)
	}

	rules := make([]*rulecond.Rule, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		rule, err := claudeRuleSource(entry.Name())
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// claudeRuleSource parses and validates one embedded rule source file. A rule
// whose trigger fields cannot compile fails generation by name rather than
// silently losing its trigger (REQ-CONDRULE-SCHEMA-03).
func claudeRuleSource(filename string) (*rulecond.Rule, error) {
	srcPath := path.Join(contentRuleDir, filename)
	raw, err := fs.ReadFile(contentfs.FS, srcPath)
	if err != nil {
		return nil, fmt.Errorf("컨텐츠 파일 읽기 실패 %s: %w", srcPath, err)
	}
	rule, err := rulecond.ParseRule(filename, raw)
	if err != nil {
		return nil, fmt.Errorf("룰 파싱 실패 %s: %w", filename, err)
	}
	if err := rulecond.Validate(rule); err != nil {
		return nil, fmt.Errorf("룰 검증 실패 %s: %w", filename, err)
	}
	return rule, nil
}

// claudeRulePaths returns the `globs` of one rule source, which claude-code
// consumes as a native `paths:` frontmatter list (REQ-CONDRULE-COMPILE-01).
func claudeRulePaths(filename string) ([]string, error) {
	rule, err := claudeRuleSource(filename)
	if err != nil {
		return nil, err
	}
	return rule.Globs, nil
}

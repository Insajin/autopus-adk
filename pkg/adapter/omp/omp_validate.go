package omp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

type ompSurfaceSet struct {
	file     string
	label    string
	expected map[string]bool
	actual   map[string]bool
}

// @AX:WARN [AUTO]: installed-surface validation has cyclomatic complexity 15.
// @AX:REASON [AUTO]: gocyclo reports 15 across cancellation, manifest, generated-file, config-marker, and stale-surface checks.
func (a *Adapter) validateInstalledSurface(ctx context.Context) ([]adapter.ValidationError, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	findings, incomplete := a.validateBaseSurface()
	if incomplete {
		return findings, nil
	}
	findings = append(findings, a.validateOMPConfigValue()...)

	rules, err := a.prepareRuleMappings()
	if err != nil {
		return findings, err
	}
	agents, err := a.prepareAgentMappings()
	if err != nil {
		return findings, err
	}
	sets := []ompSurfaceSet{
		mappingSurfaceSet(a.root, ".agents/rules/autopus", "rules", rules),
		mappingSurfaceSet(a.root, ".omp/agents", "agents", agents),
	}

	cfg, cfgErr := config.LoadPreview(a.root)
	if cfgErr == nil && ompOwnsCommandSurface(cfg) {
		commands, mapErr := a.prepareCommandMappings(cfg)
		if mapErr != nil {
			return findings, mapErr
		}
		sets = append(sets, mappingSurfaceSet(a.root, ".agents/commands", "commands", commands))
	}
	if cfgErr == nil && ompOwnsSharedSkillSurface(cfg) {
		workflow, mapErr := a.prepareWorkflowSkillMappings(cfg)
		if mapErr != nil {
			return findings, mapErr
		}
		extended, mapErr := a.prepareExtendedSkillMappings(cfg)
		if mapErr != nil {
			return findings, mapErr
		}
		workflowSet, extendedSet := splitOMPSkillSurface(a.root, workflow, extended)
		sets = append(sets, workflowSet, extendedSet)
	}

	for _, set := range sets {
		if finding := compareOMPSurfaceSet(set); finding != nil {
			findings = append(findings, *finding)
		}
	}
	return findings, nil
}

func (a *Adapter) validateBaseSurface() ([]adapter.ValidationError, bool) {
	checks := []struct{ path, message string }{
		{configFile, ".omp/config.yml이 없음"},
		{filepath.Join(".omp", "agents"), "omp agent 디렉터리가 없음"},
		{filepath.Join(".agents", "rules", "autopus"), "omp rule 디렉터리가 없음"},
	}
	var findings []adapter.ValidationError
	for _, check := range checks {
		if _, err := os.Stat(filepath.Join(a.root, check.path)); os.IsNotExist(err) {
			findings = append(findings, adapter.ValidationError{
				File: check.path, Message: check.message, Level: "error",
			})
		}
	}
	return findings, len(findings) > 0
}

func (a *Adapter) validateOMPConfigValue() []adapter.ValidationError {
	data, err := os.ReadFile(filepath.Join(a.root, configFile))
	if err != nil {
		return []adapter.ValidationError{{File: configFile, Message: err.Error(), Level: "error"}}
	}
	layout, err := parseOMPConfigLayout(string(data))
	if err != nil {
		return []adapter.ValidationError{{
			File: configFile, Message: "YAML parse/structure error: " + err.Error(), Level: "error",
		}}
	}
	got := ompCustomDirectories(layout)
	if len(got) != 1 || got[0] != ".agents/skills" {
		return []adapter.ValidationError{{
			File: configFile,
			Message: fmt.Sprintf("skills.customDirectories expected=[.agents/skills] got=[%s]",
				strings.Join(got, ",")),
			Level: "error",
		}}
	}
	return nil
}

func ompCustomDirectories(layout ompConfigLayout) []string {
	if layout.customKey == nil || layout.skillsValue == nil {
		return nil
	}
	for index, node := range layout.skillsValue.Content {
		if node != layout.customKey || index+1 >= len(layout.skillsValue.Content) {
			continue
		}
		value := layout.skillsValue.Content[index+1]
		if value.ShortTag() != "!!seq" {
			return []string{"<non-sequence>"}
		}
		directories := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.ShortTag() != "!!str" {
				return []string{"<non-string>"}
			}
			directories = append(directories, item.Value)
		}
		return directories
	}
	return nil
}

func mappingSurfaceSet(root, prefix, label string, mappings []adapter.FileMapping) ompSurfaceSet {
	expected := make(map[string]bool, len(mappings))
	for _, mapping := range mappings {
		rel, err := filepath.Rel(filepath.FromSlash(prefix), mapping.TargetPath)
		if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			expected[filepath.ToSlash(rel)] = true
		}
	}
	return ompSurfaceSet{
		file: filepath.FromSlash(prefix), label: label, expected: expected,
		actual: collectOMPRegularFiles(filepath.Join(root, filepath.FromSlash(prefix))),
	}
}

func splitOMPSkillSurface(root string, workflow, extended []adapter.FileMapping) (ompSurfaceSet, ompSurfaceSet) {
	prefix := ".agents/skills"
	workflowSet := mappingSurfaceSet(root, prefix, "workflow skills", workflow)
	extendedSet := mappingSurfaceSet(root, prefix, "compiled skills", extended)
	all := collectOMPRegularFiles(filepath.Join(root, filepath.FromSlash(prefix)))
	workflowSet.actual = make(map[string]bool)
	extendedSet.actual = make(map[string]bool)
	workflowNames := make(map[string]bool, len(workflowSpecs))
	for _, spec := range workflowSpecs {
		workflowNames[spec.Name] = true
	}
	for path := range all {
		name := strings.SplitN(path, "/", 2)[0]
		if workflowNames[name] {
			workflowSet.actual[path] = true
		} else {
			extendedSet.actual[path] = true
		}
	}
	return workflowSet, extendedSet
}

func collectOMPRegularFiles(root string) map[string]bool {
	files := make(map[string]bool)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr == nil {
			files[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	return files
}

func compareOMPSurfaceSet(set ompSurfaceSet) *adapter.ValidationError {
	missing, extra := ompSetDiff(set.expected, set.actual), ompSetDiff(set.actual, set.expected)
	if len(missing) == 0 && len(extra) == 0 {
		return nil
	}
	return &adapter.ValidationError{
		File: set.file,
		Message: fmt.Sprintf("%s expected=%d got=%d missing=[%s] extra=[%s]",
			set.label, len(set.expected), len(set.actual), strings.Join(missing, ","), strings.Join(extra, ",")),
		Level: "error",
	}
}

func ompSetDiff(left, right map[string]bool) []string {
	var values []string
	for value := range left {
		if !right[value] {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

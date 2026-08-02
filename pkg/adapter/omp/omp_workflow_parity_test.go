package omp

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func TestOMP002_WorkflowParity_GenerateEmitsCanonicalCommandsAndSkills(t *testing.T) {
	t.Parallel()

	a := NewWithRoot(t.TempDir())
	pf, err := a.Generate(context.Background(), configForOMP())
	require.NoError(t, err)

	want := make(map[string]bool, len(workflowSpecs))
	wantWorkflowTargets := make(map[string]bool, len(workflowSpecs))
	for _, spec := range workflowSpecs {
		want[spec.Name] = true
		wantWorkflowTargets[filepath.ToSlash(filepath.Join(".agents", "skills", spec.Name, "SKILL.md"))] = true
	}
	require.Len(t, want, 20, "workflowSpecs is the canonical twenty-name authority")
	workflow, err := a.prepareWorkflowSkillMappings(configForOMP())
	require.NoError(t, err)
	assert.Equal(t, wantWorkflowTargets, mappingTargetSet(workflow),
		"the workflow emitter, not the extended emitter, must own all twenty targets")
	extended, err := a.prepareExtendedSkillMappings(configForOMP())
	require.NoError(t, err)
	for _, file := range extended {
		assert.False(t, wantWorkflowTargets[filepath.ToSlash(file.TargetPath)],
			"extended emitter must exclude workflow target %q", file.TargetPath)
	}

	commands := map[string]bool{}
	skills := map[string]string{}
	seen := map[string]int{}
	for _, file := range pf.Files {
		target := filepath.ToSlash(file.TargetPath)
		seen[target]++
		if strings.HasPrefix(target, ".agents/commands/") && strings.HasSuffix(target, ".md") {
			name := strings.TrimSuffix(strings.TrimPrefix(target, ".agents/commands/"), ".md")
			if !strings.Contains(name, "/") {
				commands[name] = true
			}
		}
		const skillPrefix, skillSuffix = ".agents/skills/", "/SKILL.md"
		if strings.HasPrefix(target, skillPrefix) && strings.HasSuffix(target, skillSuffix) {
			name := strings.TrimSuffix(strings.TrimPrefix(target, skillPrefix), skillSuffix)
			if want[name] {
				skills[name] = string(file.Content)
			}
		}
	}
	for target, count := range seen {
		assert.Equal(t, 1, count, "emitted target %q must have duplicate count zero", target)
	}
	assert.Equal(t, want, commands, "command basenames must equal workflowSpecs")
	assert.Equal(t, want, mappingContentKeys(skills),
		"workflow skill basenames must equal workflowSpecs; extended skills are excluded")

	router, ok := skills["auto"]
	if assert.True(t, ok, "the thin auto router skill must be emitted") {
		frontmatter, body := splitEmittedFrontmatter(t, router)
		assert.Contains(t, frontmatter, "name: auto")
		assert.Contains(t, body, "# Autopus 명령 라우터")
		assert.NotContains(t, body, "## Context Profile", "the router must not impersonate a detail skill")
	}
	for _, spec := range workflowSpecs[1:] {
		content, ok := skills[spec.Name]
		if !assert.True(t, ok, "detailed workflow skill %s must be emitted", spec.Name) {
			continue
		}
		frontmatter, body := splitEmittedFrontmatter(t, content)
		assert.Contains(t, frontmatter, "name: "+spec.Name)
		assert.NotEmpty(t, strings.TrimSpace(body))
		assert.Contains(t, body, "## ", "%s needs structured execution sections", spec.Name)
		assert.NotContains(t, body, "# Autopus 명령 라우터")
	}
}

func TestOMP002_RoutingContract_RouterAndAliasesResolveExactDetails(t *testing.T) {
	t.Parallel()

	a := NewWithRoot(t.TempDir())
	routerCommand, err := a.renderRouterCommand()
	require.NoError(t, err)
	routerSkill, err := a.renderRouterSkill()
	require.NoError(t, err)

	for _, spec := range workflowSpecs[1:] {
		spec := spec
		t.Run(spec.Name, func(t *testing.T) {
			t.Parallel()
			alias, renderErr := a.renderWorkflowCommand(spec, configForOMP())
			require.NoError(t, renderErr)
			for _, entry := range []struct {
				name        string
				content     string
				wantNoFuzzy bool
			}{
				{name: "router command", content: routerCommand, wantNoFuzzy: true},
				{name: "router skill", content: routerSkill, wantNoFuzzy: true},
				{name: "direct alias", content: alias},
			} {
				t.Run(entry.name, func(t *testing.T) {
					assert.Equal(t, 1, strings.Count(entry.content, "`"+spec.Name+"`"),
						"entrypoint must route to one exact detail target")
					for _, placeholder := range []string{"$ARGUMENTS", "--model <provider/model>", "--variant <value>"} {
						assert.Equal(t, 1, strings.Count(entry.content, placeholder),
							"%s must preserve %s exactly once", entry.name, placeholder)
					}
					if entry.wantNoFuzzy {
						assert.Contains(t, strings.ToLower(entry.content), "do not fuzzy-correct")
					}
				})
			}
		})
	}
}

func mappingContentKeys(files map[string]string) map[string]bool {
	out := make(map[string]bool, len(files))
	for name := range files {
		out[name] = true
	}
	return out
}

func TestOMP002_S5_ConfigUsesBothStructuralMarkerFormsAndFailsClosed(t *testing.T) {
	tests := []struct {
		name     string
		original string
		want     []string
	}{
		{
			name:     "no user skills mapping owns top-level subtree",
			original: "model: user-model\n",
			want:     []string{"model: user-model\n", "# AUTOPUS:BEGIN\nskills:\n  customDirectories:\n    - .agents/skills\n# AUTOPUS:END"},
		},
		{
			name:     "existing user skills mapping owns only indented entry",
			original: "model: user-model\nskills:\n  userDirectory: ./private\n",
			want: []string{
				"model: user-model\nskills:\n  userDirectory: ./private\n",
				"  # AUTOPUS:BEGIN\n  customDirectories:\n    - .agents/skills\n  # AUTOPUS:END",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, configFile)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, []byte(tc.original), 0o600))

			_, err := NewWithRoot(dir).Generate(context.Background(), configForOMP())
			require.NoError(t, err)
			after, err := os.ReadFile(path)
			require.NoError(t, err)
			for _, fragment := range tc.want {
				assert.Contains(t, string(after), fragment)
			}
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	}

	conflicts := []string{
		"skills: scalar-owned-by-user\n",
		"skills:\n  customDirectories:\n    - ./user-owned\n",
	}
	for _, original := range conflicts {
		t.Run("conflict remains byte identical", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, configFile)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

			_, err := NewWithRoot(dir).Generate(context.Background(), configForOMP())
			require.Error(t, err)
			after, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, original, string(after))
			info, statErr := os.Stat(path)
			require.NoError(t, statErr)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	}
}

func TestOMP002_S7_ValidateReportsMalformedValuesAndSourceDerivedMissingSets(t *testing.T) {
	dir := generateOMPOnly(t)
	a := NewWithRoot(dir)
	cfg := configForOMP()

	rules, err := a.prepareRuleMappings()
	require.NoError(t, err)
	agents, err := a.prepareAgentMappings()
	require.NoError(t, err)
	commands, err := a.prepareCommandMappings(cfg)
	require.NoError(t, err)
	removeStableMapping(t, dir, rules)
	removeStableMapping(t, dir, agents)
	removeStableMapping(t, dir, commands)
	_ = os.Remove(filepath.Join(dir, ".agents", "skills", "auto-plan", "SKILL.md"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFile), []byte("skills: [\n"), 0o600))

	findings, err := a.Validate(context.Background())
	require.NoError(t, err)
	var details []string
	for _, finding := range findings {
		details = append(details, finding.File+" "+finding.Message)
	}
	joined := strings.Join(details, "\n")
	assert.Contains(t, strings.ToLower(joined), "yaml")
	assert.Contains(t, joined, "expected="+itoa(len(rules))+" got="+itoa(len(rules)-1))
	assert.Contains(t, joined, "expected="+itoa(len(agents))+" got="+itoa(len(agents)-1))
	assert.GreaterOrEqual(t, strings.Count(joined, "expected=20 got=19"), 2,
		"commands and workflow skills require separate source-derived findings")
	assert.Contains(t, joined, "auto-plan")
}

func TestOMP002_S9_DetectRequiresExactSemverAndHonorsCallerDeadline(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	writeFakeOMPBinary(t, "omp/not-semver")
	got, err := NewWithRoot(t.TempDir()).Detect(context.Background())
	require.NoError(t, err)
	assert.False(t, got, "an omp/ prefix without a semantic version is not readiness authority")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, cliBinary),
		[]byte("#!/bin/sh\n/bin/sleep 5\nprintf 'omp/17.1.8\\n'\n"), 0o755))
	t.Setenv("PATH", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	got, err = NewWithRoot(t.TempDir()).Detect(ctx)
	require.NoError(t, err)
	assert.False(t, got)
	assert.Less(t, time.Since(started), time.Second, "readiness identity probe must be bounded")
}

func mappingTargetSet(files []adapter.FileMapping) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, file := range files {
		out[filepath.ToSlash(file.TargetPath)] = true
	}
	return out
}

func removeStableMapping(t *testing.T, root string, files []adapter.FileMapping) {
	t.Helper()
	require.NotEmpty(t, files)
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.TargetPath)
	}
	sort.Strings(paths)
	require.NoError(t, os.Remove(filepath.Join(root, paths[0])))
}

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var out []byte
	for value > 0 {
		out = append([]byte{digits[value%10]}, out...)
		value /= 10
	}
	return string(out)
}

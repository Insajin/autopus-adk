package gemini

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var frozenGeminiAutoRoutes = []string{
	"setup", "status", "goal", "update", "plan", "go", "fix", "review", "sync",
	"idea", "map", "why", "verify", "secure", "test", "qa", "dev", "canary", "doctor",
}

func TestRouterBudget_FullGenerate_RootIsThinAndAllDetailsExist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("router-budget"))
	require.NoError(t, err)

	router := readGeneratedGeminiSurface(t, root, filepath.Join(".gemini", "skills", "auto", "SKILL.md"))
	t.Logf("generated Gemini root router: %d bytes", len([]byte(router)))
	assert.LessOrEqual(t, len([]byte(router)), 8192, "root router must stay within the byte budget")
	for _, token := range []string{
		"Language Policy", "Source Ownership", "Subagent Delegation", "Review Convergence",
		"Generated Surface Safety", ".autopus/project/workspace.md",
	} {
		assert.Contains(t, router, token)
	}
	for _, route := range frozenGeminiAutoRoutes {
		rel := filepath.Join(".gemini", "skills", "autopus", "auto-"+route, "SKILL.md")
		assert.Equal(t, 1, strings.Count(router, rel), "route %q must resolve exactly one detail", route)
		detail := readGeneratedGeminiSurface(t, root, rel)
		assert.Contains(t, detail, "name: auto-"+route)
	}
	for _, alias := range []string{"browse", "stale", "spec review", "init", "platform"} {
		assert.Contains(t, router, alias, "legacy alias %q must remain routable", alias)
	}
	assert.NotContains(t, router, "Triage Process")
}

func TestWorkflowSkills_MissingElevenRoutes_AreBoundedContracts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("route-gaps"))
	require.NoError(t, err)

	missingBeforeT5 := []string{"setup", "status", "goal", "update", "map", "why", "verify", "secure", "test", "dev", "doctor"}
	for _, route := range missingBeforeT5 {
		rel := filepath.Join(".gemini", "skills", "autopus", "auto-"+route, "SKILL.md")
		body := readGeneratedGeminiSurface(t, root, rel)
		assert.LessOrEqual(t, len([]byte(body)), 8192, "%s detail must remain bounded", route)
		assert.Contains(t, body, "## Context Profile")
		assert.Contains(t, body, "## Contract")
	}
}

func TestWorkflowSkills_UpdateAndGenerate_ProduceMatchingDetails(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("route-parity")
	generateRoot := t.TempDir()
	updateRoot := t.TempDir()

	_, err := NewWithRoot(generateRoot).Generate(context.Background(), cfg)
	require.NoError(t, err)
	_, err = NewWithRoot(updateRoot).Update(context.Background(), cfg)
	require.NoError(t, err)

	for _, route := range frozenGeminiAutoRoutes {
		rel := filepath.Join(".gemini", "skills", "autopus", "auto-"+route, "SKILL.md")
		assert.Equal(t,
			readGeneratedGeminiSurface(t, generateRoot, rel),
			readGeneratedGeminiSurface(t, updateRoot, rel),
			rel,
		)
	}
}

func TestWorkflowSkills_GenerateAndUpdate_DoNotDuplicateCanonicalRouteTargets(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("route-ownership")
	for _, operation := range []struct {
		name string
		run  func(string) ([]string, error)
	}{
		{name: "generate", run: func(root string) ([]string, error) {
			files, err := NewWithRoot(root).Generate(context.Background(), cfg)
			if err != nil {
				return nil, err
			}
			return geminiMappingPaths(files.Files), nil
		}},
		{name: "update", run: func(root string) ([]string, error) {
			files, err := NewWithRoot(root).Update(context.Background(), cfg)
			if err != nil {
				return nil, err
			}
			return geminiMappingPaths(files.Files), nil
		}},
	} {
		operation := operation
		t.Run(operation.name, func(t *testing.T) {
			paths, err := operation.run(t.TempDir())
			require.NoError(t, err)
			for _, target := range []string{
				filepath.Join(".gemini", "skills", "autopus", "auto-setup", "SKILL.md"),
				filepath.Join(antigravityPluginDir, "skills", "auto-setup", "SKILL.md"),
			} {
				assert.Equal(t, 1, countGeminiPath(paths, target), target)
			}
		})
	}
}

func TestWorkflowSkills_GenerateAndUpdate_RespectRouteAndSharedSkillOwnership(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("skill-ownership")
	operations := []struct {
		name string
		run  func(string) (*adapter.PlatformFiles, error)
	}{
		{name: "generate", run: func(root string) (*adapter.PlatformFiles, error) {
			return NewWithRoot(root).Generate(context.Background(), cfg)
		}},
		{name: "update", run: func(root string) (*adapter.PlatformFiles, error) {
			return NewWithRoot(root).Update(context.Background(), cfg)
		}},
	}

	for _, operation := range operations {
		operation := operation
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			files, err := operation.run(root)
			require.NoError(t, err)

			paths := geminiMappingPaths(files.Files)
			seen := make(map[string]struct{}, len(paths))
			for _, path := range paths {
				_, duplicate := seen[path]
				assert.False(t, duplicate, "duplicate target path: %s", path)
				seen[path] = struct{}{}
			}

			for _, base := range []string{
				filepath.Join(".gemini", "skills", "autopus"),
				filepath.Join(antigravityPluginDir, "skills"),
			} {
				adaptivePath := filepath.Join(base, "adaptive-quality", "SKILL.md")
				autoGoPath := filepath.Join(base, "auto-go", "SKILL.md")
				assert.Equal(t, 1, countGeminiPath(paths, adaptivePath), adaptivePath)
				assert.Equal(t, 1, countGeminiPath(paths, autoGoPath), autoGoPath)

				adaptive := readGeneratedGeminiSurface(t, root, adaptivePath)
				autoGo := readGeneratedGeminiSurface(t, root, autoGoPath)
				assert.Equal(t, "adaptive-quality", generatedGeminiSkillName(adaptivePath, adaptive))
				assert.Equal(t, "auto-go", generatedGeminiSkillName(autoGoPath, autoGo))
				assert.Contains(t, autoGo, "\nplatform: antigravity-cli\n")
			}
		})
	}
}

func generatedGeminiSkillName(path, content string) string {
	if strings.HasPrefix(content, "---\n") {
		if end := strings.Index(content[4:], "\n---"); end >= 0 {
			for _, line := range strings.Split(content[4:4+end], "\n") {
				if value, ok := strings.CutPrefix(strings.TrimSpace(line), "name:"); ok {
					return strings.TrimSpace(value)
				}
			}
		}
	}
	return filepath.Base(filepath.Dir(path))
}

func geminiMappingPaths(files []adapter.FileMapping) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, filepath.Clean(file.TargetPath))
	}
	return paths
}

func countGeminiPath(paths []string, target string) int {
	count := 0
	for _, path := range paths {
		if path == filepath.Clean(target) {
			count++
		}
	}
	return count
}

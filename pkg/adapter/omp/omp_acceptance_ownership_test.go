package omp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

// generateForPlatforms persists the platform list and runs omp Generate.
func generateForPlatforms(t *testing.T, platforms ...string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("omp-acceptance")
	cfg.Platforms = platforms
	require.NoError(t, config.Save(dir, cfg))
	_, err := NewWithRoot(dir).Generate(context.Background(), cfg)
	require.NoError(t, err)
	return dir
}

func skillDirNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".agents", "skills"))
	if os.IsNotExist(err) {
		return map[string]bool{}
	}
	require.NoError(t, err)
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names[e.Name()] = true
		}
	}
	return names
}

func commandFileNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".agents", "commands"))
	if os.IsNotExist(err) {
		return map[string]bool{}
	}
	require.NoError(t, err)
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names[e.Name()] = true
		}
	}
	return names
}

// TestOMPAcceptance_S10_SkillSurfaceYield covers REQ-013 across four configurations.
func TestOMPAcceptance_S10_SkillSurfaceYield(t *testing.T) {
	dirA := generateForPlatforms(t, "omp")
	namesA := skillDirNames(t, dirA)
	assert.True(t, namesA["auto"], "config A must emit the auto skill")
	assert.False(t, namesA["harness-workflow"],
		"harness-workflow declares platforms without omp")
	assert.False(t, namesA["agent-teams"],
		"agent-teams declares platforms without omp")

	dirD := generateForPlatforms(t, "antigravity-cli", "omp")
	assert.Equal(t, namesA, skillDirNames(t, dirD),
		"gemini mirrors skills elsewhere, so config D must match config A")

	for _, owner := range []string{"opencode", "codex"} {
		t.Run("yield to "+owner, func(t *testing.T) {
			dir := t.TempDir()
			cfg := config.DefaultFullConfig("omp-acceptance")
			cfg.Platforms = []string{owner, "omp"}
			require.NoError(t, config.Save(dir, cfg))

			existing := filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md")
			require.NoError(t, os.MkdirAll(filepath.Dir(existing), 0o755))
			require.NoError(t, os.WriteFile(existing, []byte(owner+" owned skill\n"), 0o644))

			_, err := NewWithRoot(dir).Generate(context.Background(), cfg)
			require.NoError(t, err)

			assert.Equal(t, 0, countPathsWithPrefix(manifestPaths(t, dir), ".agents/skills/"),
				"omp manifest must not record a yielded skill path")
			after, err := os.ReadFile(existing)
			require.NoError(t, err)
			assert.Equal(t, owner+" owned skill\n", string(after))
		})
	}
}

// TestOMPAcceptance_S11_CommandSurfaceOwnership covers REQ-014.
func TestOMPAcceptance_S11_CommandSurfaceOwnership(t *testing.T) {
	dirA := generateForPlatforms(t, "omp")
	namesA := commandFileNames(t, dirA)

	want := make(map[string]bool, len(workflowSpecs))
	for _, spec := range workflowSpecs {
		want[spec.Name+".md"] = true
	}
	assert.Len(t, want, 20, "workflowSpecs must hold exactly 20 commands")
	assert.Equal(t, want, namesA, ".agents/commands/ name set must equal workflowSpecs")

	for name := range namesA {
		data, err := os.ReadFile(filepath.Join(dirA, ".agents", "commands", name))
		require.NoError(t, err)
		_, body := splitEmittedFrontmatter(t, string(data))
		assert.NotEmpty(t, strings.TrimSpace(body), "command %s must carry a body", name)
		if name == "auto.md" {
			assert.NotContains(t, body, "auto pipeline run",
				"interactive /auto must not invoke the headless RPC backend")
			assert.NotContains(t, body, "spawn(",
				"interactive /auto must use the current OMP session")
		}
	}
	assert.NoFileExists(t, filepath.Join(dirA, filepath.FromSlash(ompNativePipelineRouteTarget)),
		"the explicit /autopus-pipeline extension must remain opt-in")

	// Config B: opencode owns commands via .opencode/commands/.
	dirB := generateForPlatforms(t, "opencode", "omp")
	assert.Equal(t, 0, countPathsWithPrefix(manifestPaths(t, dirB), ".agents/commands/"),
		"omp must yield the command surface to opencode")
	assert.Empty(t, commandFileNames(t, dirB))
}

// TestOMPAcceptance_S11_AntigravityCoexistence covers REQ-014 md/toml coexistence.
func TestOMPAcceptance_S11_AntigravityCoexistence(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("omp-acceptance")
	cfg.Platforms = []string{"antigravity-cli", "omp"}
	require.NoError(t, config.Save(dir, cfg))

	// gemini writes .toml plus an auto/ subtree in the same directory.
	geminiToml := filepath.Join(dir, ".agents", "commands", "auto.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(geminiToml), 0o755))
	require.NoError(t, os.WriteFile(geminiToml, []byte("prompt = \"gemini\"\n"), 0o644))
	geminiSub := filepath.Join(dir, ".agents", "commands", "auto", "plan.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(geminiSub), 0o755))
	require.NoError(t, os.WriteFile(geminiSub, []byte("prompt = \"plan\"\n"), 0o644))

	a := NewWithRoot(dir)
	ctx := context.Background()
	_, err := a.Generate(ctx, cfg)
	require.NoError(t, err)

	assert.Len(t, commandFileNames(t, dir), 20,
		"omp must still emit 20 .md commands alongside gemini .toml files")
	for _, p := range manifestPaths(t, dir) {
		assert.False(t, strings.HasSuffix(p, ".toml"),
			"omp manifest must not record a .toml path, found %q", p)
	}

	require.NoError(t, a.Clean(ctx))

	tomlAfter, err := os.ReadFile(geminiToml)
	require.NoError(t, err, "gemini auto.toml must survive omp Clean")
	assert.Equal(t, "prompt = \"gemini\"\n", string(tomlAfter))
	subAfter, err := os.ReadFile(geminiSub)
	require.NoError(t, err, "gemini auto/plan.toml must survive omp Clean")
	assert.Equal(t, "prompt = \"plan\"\n", string(subAfter))
}

// TestOMPAcceptance_E4_OwnershipTransitionUpdate covers REQ-017 orphan removal
// when a newly added platform takes over a surface omp previously owned.
func TestOMPAcceptance_E4_OwnershipTransitionUpdate(t *testing.T) {
	dir := generateForPlatforms(t, "omp")

	before := manifestPaths(t, dir)
	require.Equal(t, 20, countPathsWithPrefix(before, ".agents/commands/"))
	require.Greater(t, countPathsWithPrefix(before, ".agents/skills/"), 0)

	cfg := config.DefaultFullConfig("omp-acceptance")
	cfg.Platforms = []string{"opencode", "omp"}
	require.NoError(t, config.Save(dir, cfg))

	a := NewWithRoot(dir)
	ctx := context.Background()
	_, err := a.Update(ctx, cfg)
	require.NoError(t, err)

	after := manifestPaths(t, dir)
	assert.Equal(t, 0, countPathsWithPrefix(after, ".agents/skills/"),
		"manifest must drop yielded skill paths")
	assert.Equal(t, 0, countPathsWithPrefix(after, ".agents/commands/"),
		"manifest must drop yielded command paths")

	// opencode serves commands from .opencode/commands/, so the .agents/commands/
	// files omp created are orphans and must be removed.
	assert.Empty(t, commandFileNames(t, dir),
		"omp-created and unmodified .agents/commands/*.md must be removed on ownership transfer")

	// opencode does write .agents/skills/, so omp Clean must leave that surface alone.
	skillsBefore := snapshotSkillFiles(t, dir)
	require.NoError(t, a.Clean(ctx))
	assert.Equal(t, skillsBefore, snapshotSkillFiles(t, dir),
		"omp Clean must not delete any .agents/skills/ file after yielding")
}

func snapshotSkillFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	base := filepath.Join(root, ".agents", "skills")
	out := make(map[string]string)
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return relErr
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	require.NoError(t, err)
	return out
}

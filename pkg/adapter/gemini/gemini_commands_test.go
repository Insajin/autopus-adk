package gemini_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestGeminiGenerateCommands(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Verify auto.toml is created
	autoPath := filepath.Join(dir, ".gemini", "commands", "auto.toml")
	_, autoErr := os.Stat(autoPath)
	assert.NoError(t, autoErr, "auto.toml should exist")

	// Verify all 6 command TOML files are created
	commands := []string{"plan", "go", "fix", "review", "sync", "idea"}
	for _, cmd := range commands {
		cmdPath := filepath.Join(dir, ".gemini", "commands", "auto", cmd+".toml")
		_, statErr := os.Stat(cmdPath)
		assert.NoError(t, statErr, "command file should exist: %s.toml", cmd)
	}
}

func TestGeminiCommandTOMLContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Check auto.toml has prompt field and router skill reference
	autoPath := filepath.Join(dir, ".gemini", "commands", "auto.toml")
	autoData, err := os.ReadFile(autoPath)
	require.NoError(t, err)
	autoContent := string(autoData)
	assert.Contains(t, autoContent, "prompt", "auto.toml should have prompt field")
	assert.Contains(t, autoContent, ".gemini/skills/auto/SKILL.md", "auto.toml should reference auto router skill")
	assert.Contains(t, autoContent, "test-project", "auto.toml should contain project name")

	// Check plan.toml has prompt field and skill reference
	planPath := filepath.Join(dir, ".gemini", "commands", "auto", "plan.toml")
	data, err := os.ReadFile(planPath)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "prompt", "should have prompt field")
	assert.Contains(t, content, ".gemini/skills/autopus/auto-plan/SKILL.md",
		"should reference auto-plan skill")
	assert.Contains(t, content, "test-project", "should contain project name")

	// Check go.toml references auto-go skill
	goPath := filepath.Join(dir, ".gemini", "commands", "auto", "go.toml")
	goData, err := os.ReadFile(goPath)
	require.NoError(t, err)
	assert.Contains(t, string(goData), "auto-go/SKILL.md",
		"go.toml should reference auto-go skill")
}

package content_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

const executorMD = `---
name: executor
description: TDD/DDD implementation agent
model: sonnet
tools: Read, Write, Edit, Grep, Glob, Bash, TodoWrite
permissionMode: acceptEdits
maxTurns: 50
skills:
  - tdd
  - ddd
  - debugging
---

# Executor Agent

## Role

Implements code following TDD methodology.

## Workflow

1. Read SPEC requirements
2. Run failing tests
3. Write implementation
4. Refactor

## Constraints

- File size limit: 300 lines
- Use Agent(subagent_type="tester", task="run tests") for delegation
- Check .claude/rules/ for guidelines
- Use mcp__context7__resolve-library-id for docs

## Completion Criteria

All tests pass with 85%+ coverage.
`

func TestParseAgentSource_Frontmatter(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"agents/executor.md": {Data: []byte(executorMD)},
	}
	sources, err := content.LoadAgentSourcesFromFS(fsys, "agents")
	require.NoError(t, err)
	require.Len(t, sources, 1)

	src := sources[0]
	assert.Equal(t, "executor", src.Meta.Name)
	assert.Equal(t, "TDD/DDD implementation agent", src.Meta.Description)
	assert.Equal(t, "sonnet", src.Meta.Model)
	assert.Equal(t, "Read, Write, Edit, Grep, Glob, Bash, TodoWrite", src.Meta.Tools)
	assert.Equal(t, "acceptEdits", src.Meta.PermissionMode)
	assert.Equal(t, 50, src.Meta.MaxTurns)
	assert.Equal(t, []string{"tdd", "ddd", "debugging"}, src.Meta.Skills)
}

func TestTransformAgentForCodex_RichInstructions(t *testing.T) {
	t.Parallel()

	src := makeExecutorSource()
	result := content.TransformAgentForCodex(src)

	// S1: TOML with developer_instructions >= 200 chars
	assert.Contains(t, result, `name = "executor"`)
	assert.Contains(t, result, `model = "gpt-5.4"`)
	assert.Contains(t, result, "[developer_instructions]")
	assert.Contains(t, result, "text = ")
	assert.Contains(t, result, "{{.ProjectName}}")
	assert.Contains(t, result, "{{if .IsFullMode}}")
	assert.Greater(t, len(result), 200)
}

func TestTransformAgentForGemini_Sections(t *testing.T) {
	t.Parallel()

	src := makeExecutorSource()
	result := content.TransformAgentForGemini(src)

	// S2: Gemini MD with role/workflow/constraints sections
	assert.Contains(t, result, "name: auto-agent-executor")
	assert.Contains(t, result, "description: TDD/DDD implementation agent")
	assert.Contains(t, result, "## Role")
	assert.Contains(t, result, "## Workflow")
	assert.Contains(t, result, "## Constraints")
	assert.Contains(t, result, "skills:")
	assert.Contains(t, result, "  - tdd")
}

func TestTransformAgentForCodex_EmptyBody(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "minimal",
			Description: "Minimal agent",
			Model:       "haiku",
		},
	}
	result := content.TransformAgentForCodex(src)

	assert.Contains(t, result, `name = "minimal"`)
	assert.Contains(t, result, `model = "gpt-5-nano"`)
	assert.Contains(t, result, "[developer_instructions]")
}

func TestTransformAgentForGemini_EmptyBody(t *testing.T) {
	t.Parallel()

	src := content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        "minimal",
			Description: "Minimal agent",
			Model:       "haiku",
		},
	}
	result := content.TransformAgentForGemini(src)

	assert.Contains(t, result, "name: auto-agent-minimal")
	assert.Contains(t, result, "description: Minimal agent")
}

func TestParseAgentSource_MissingFrontmatterFields(t *testing.T) {
	t.Parallel()

	md := `---
name: sparse
---

# Sparse Agent

Just a simple body.
`
	fsys := fstest.MapFS{
		"agents/sparse.md": {Data: []byte(md)},
	}
	sources, err := content.LoadAgentSourcesFromFS(fsys, "agents")
	require.NoError(t, err)
	require.Len(t, sources, 1)

	src := sources[0]
	assert.Equal(t, "sparse", src.Meta.Name)
	assert.Empty(t, src.Meta.Description)
	assert.Empty(t, src.Meta.Model)
	assert.Empty(t, src.Meta.Tools)
	assert.Zero(t, src.Meta.MaxTurns)
	assert.Nil(t, src.Meta.Skills)
}

func TestParseAgentSource_NoFrontmatter(t *testing.T) {
	t.Parallel()

	md := `# No Frontmatter Agent

Just body content.
`
	fsys := fstest.MapFS{
		"agents/nofm.md": {Data: []byte(md)},
	}
	sources, err := content.LoadAgentSourcesFromFS(fsys, "agents")
	require.NoError(t, err)
	require.Len(t, sources, 1)

	assert.Equal(t, "nofm", sources[0].Meta.Name)
	assert.Contains(t, sources[0].Body, "No Frontmatter Agent")
}

func TestLoadAgentSourcesFromFS_MultipleFiles(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"agents/a.md":       {Data: []byte("---\nname: alpha\n---\nBody A")},
		"agents/b.md":       {Data: []byte("---\nname: beta\n---\nBody B")},
		"agents/readme.txt": {Data: []byte("ignored")},
	}
	sources, err := content.LoadAgentSourcesFromFS(fsys, "agents")
	require.NoError(t, err)
	assert.Len(t, sources, 2)
}

func TestNewAgentTransformer(t *testing.T) {
	t.Parallel()

	sources := []content.AgentSource{
		{Meta: content.AgentSourceMeta{Name: "a"}, Body: "body a"},
		{Meta: content.AgentSourceMeta{Name: "b"}, Body: "body b"},
	}
	transformer := content.NewAgentTransformer(sources)
	assert.Len(t, transformer.Sources(), 2)
}

// makeExecutorSource creates a test AgentSource resembling executor.md.
func makeExecutorSource() content.AgentSource {
	return content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:           "executor",
			Description:    "TDD/DDD implementation agent",
			Model:          "sonnet",
			Tools:          "Read, Write, Edit, Grep, Glob, Bash, TodoWrite",
			PermissionMode: "acceptEdits",
			MaxTurns:       50,
			Skills:         []string{"tdd", "ddd", "debugging"},
		},
		Body: `# Executor Agent

## Role

Implements code following TDD methodology.

## Workflow

1. Read SPEC requirements
2. Run failing tests
3. Write implementation
4. Refactor

## Constraints

- File size limit: 300 lines
- Use Agent(subagent_type="tester", task="run tests") for delegation
- Check .claude/rules/ for guidelines
- Use mcp__context7__resolve-library-id for docs

## Completion Criteria

All tests pass with 85%+ coverage.`,
	}
}

package opencode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitFrontmatter_NoLeadingDelimiter(t *testing.T) {
	t.Parallel()
	fm, body := splitFrontmatter("no frontmatter here")
	assert.Empty(t, fm)
	assert.Equal(t, "no frontmatter here", body)
}

func TestSplitFrontmatter_UnterminatedDelimiter(t *testing.T) {
	t.Parallel()
	content := "---\nname: x\nbody without closing"
	fm, body := splitFrontmatter(content)
	assert.Empty(t, fm)
	assert.Equal(t, content, body)
}

func TestSplitFrontmatter_Valid(t *testing.T) {
	t.Parallel()
	fm, body := splitFrontmatter("---\nname: x\n---\nhello")
	assert.Equal(t, "name: x", fm)
	assert.Equal(t, "hello", body)
}

func TestBuildMarkdown_EmptyFrontmatter(t *testing.T) {
	t.Parallel()
	out := buildMarkdown("   ", "  body  ")
	assert.Equal(t, "body\n", out)
}

func TestBuildMarkdown_WithFrontmatter(t *testing.T) {
	t.Parallel()
	out := buildMarkdown("name: x", "body")
	assert.Equal(t, "---\nname: x\n---\n\nbody\n", out)
}

func TestInjectOpenCodeBrandingBlock_AlreadyPresent(t *testing.T) {
	t.Parallel()
	body := "# Title\n\n## Autopus Branding\nexisting"
	assert.Equal(t, body, injectOpenCodeBrandingBlock(body))
}

func TestInjectOpenCodeBrandingBlock_InjectsAfterHeading(t *testing.T) {
	t.Parallel()
	out := injectOpenCodeBrandingBlock("# Title\n\nintro")
	assert.Contains(t, out, "## Autopus Branding")
	headingIdx := strings.Index(out, "# Title")
	brandIdx := strings.Index(out, "## Autopus Branding")
	assert.Less(t, headingIdx, brandIdx)
}

func TestInjectAfterFirstHeading_NoHeading(t *testing.T) {
	t.Parallel()
	out := injectAfterFirstHeading("plain body", "BLOCK")
	assert.Equal(t, "BLOCK\n\nplain body", out)
}

func TestInjectAfterFirstHeading_WithHeading(t *testing.T) {
	t.Parallel()
	out := injectAfterFirstHeading("# H\nrest", "BLOCK")
	assert.Equal(t, "# H\n\nBLOCK\n\nrest", out)
}

func TestReadJSONObject_Missing(t *testing.T) {
	t.Parallel()
	obj, err := readJSONObject(filepath.Join(t.TempDir(), "nope.json"))
	require.NoError(t, err)
	assert.Empty(t, obj)
}

func TestReadJSONObject_EmptyFile(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "empty.json")
	require.NoError(t, os.WriteFile(p, []byte("   \n"), 0o644))
	obj, err := readJSONObject(p)
	require.NoError(t, err)
	assert.Empty(t, obj)
}

func TestReadJSONObject_NullContent(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "null.json")
	require.NoError(t, os.WriteFile(p, []byte("null"), 0o644))
	obj, err := readJSONObject(p)
	require.NoError(t, err)
	assert.NotNil(t, obj)
	assert.Empty(t, obj)
}

func TestReadJSONObject_Invalid(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(p, []byte("{broken"), 0o644))
	_, err := readJSONObject(p)
	assert.Error(t, err)
}

func TestReadJSONObject_Valid(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "ok.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"a":1}`), 0o644))
	obj, err := readJSONObject(p)
	require.NoError(t, err)
	assert.Contains(t, obj, "a")
}

func TestJSONStringSlice_NonArray(t *testing.T) {
	t.Parallel()
	assert.Nil(t, jsonStringSlice("x"))
}

func TestJSONStringSlice_MixedTypes(t *testing.T) {
	t.Parallel()
	out := jsonStringSlice([]any{"a", 1, "b"})
	assert.Equal(t, []string{"a", "b"}, out)
}

func TestJSONPluginSlice_NonArray(t *testing.T) {
	t.Parallel()
	assert.Nil(t, jsonPluginSlice(42))
}

func TestJSONPluginSlice_StringsAndNestedArrays(t *testing.T) {
	t.Parallel()
	value := []any{
		"plain",
		[]any{"nested-name", "version"},
		[]any{},   // empty nested -> skipped
		[]any{42}, // non-string head -> skipped
	}
	out := jsonPluginSlice(value)
	assert.Equal(t, []string{"plain", "nested-name"}, out)
}

func TestUniqueStrings_Dedup(t *testing.T) {
	t.Parallel()
	out := uniqueStrings([]string{"a", "b"}, []string{"b", "c"})
	assert.Equal(t, []string{"a", "b", "c"}, out)
}

func TestContainsString(t *testing.T) {
	t.Parallel()
	assert.True(t, containsString([]string{"a", "b"}, "b"))
	assert.False(t, containsString([]string{"a"}, "z"))
}

func TestSkillCompilerExplicitlySelects_NilConfig(t *testing.T) {
	t.Parallel()
	assert.False(t, skillCompilerExplicitlySelects(nil, "any"))
}

func TestSkillCompilerExplicitlySelects_Matches(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("demo")
	cfg.Skills.Compiler.ExplicitSkills = []string{"alpha", "beta"}
	assert.True(t, skillCompilerExplicitlySelects(cfg, "beta"))
	assert.False(t, skillCompilerExplicitlySelects(cfg, "gamma"))
}

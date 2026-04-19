package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSkillFrontmatter_Valid verifies that a well-formed skill
// frontmatter passes validation without error.
func TestValidateSkillFrontmatter_Valid(t *testing.T) {
	t.Parallel()

	content := []byte("---\nname: my-skill\ndescription: Does something\ntriggers:\n  - /foo\n---\n")

	err := validateSkillFrontmatter(content)
	assert.NoError(t, err)
}

// TestValidateSkillFrontmatter_MissingName verifies that missing name field
// returns an appropriate error.
func TestValidateSkillFrontmatter_MissingName(t *testing.T) {
	t.Parallel()

	content := []byte("---\ndescription: Does something\ntriggers:\n  - /foo\n---\n")

	err := validateSkillFrontmatter(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

// TestValidateSkillFrontmatter_MissingDescription verifies that missing
// description field returns an error.
func TestValidateSkillFrontmatter_MissingDescription(t *testing.T) {
	t.Parallel()

	content := []byte("---\nname: my-skill\ntriggers:\n  - /foo\n---\n")

	err := validateSkillFrontmatter(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

// TestValidateSkillFrontmatter_MissingTriggers verifies that empty triggers
// list returns an error.
func TestValidateSkillFrontmatter_MissingTriggers(t *testing.T) {
	t.Parallel()

	content := []byte("---\nname: my-skill\ndescription: Desc\ntriggers: []\n---\n")

	err := validateSkillFrontmatter(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "triggers")
}

// TestValidateSkillFrontmatter_NoDelimiter verifies that content without ---
// delimiters returns an error.
func TestValidateSkillFrontmatter_NoDelimiter(t *testing.T) {
	t.Parallel()

	content := []byte("name: my-skill\ndescription: Desc\n")

	err := validateSkillFrontmatter(content)
	require.Error(t, err)
}

// TestExtractFrontmatter_ValidContent verifies that extractFrontmatter returns
// the YAML bytes between the first --- delimiters.
func TestExtractFrontmatter_ValidContent(t *testing.T) {
	t.Parallel()

	content := []byte("---\nname: test\ndescription: hello\n---\nBody here")

	fm, err := extractFrontmatter(content)
	require.NoError(t, err)

	assert.Contains(t, string(fm), "name: test")
	assert.Contains(t, string(fm), "description: hello")
	assert.NotContains(t, string(fm), "Body here")
}

// TestExtractFrontmatter_NoOpenDelimiter verifies that content without a
// leading --- returns an error.
func TestExtractFrontmatter_NoOpenDelimiter(t *testing.T) {
	t.Parallel()

	content := []byte("name: test\n---\n")

	_, err := extractFrontmatter(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frontmatter")
}

// TestExtractFrontmatter_NoCloseDelimiter verifies that content without a
// closing --- returns an error.
func TestExtractFrontmatter_NoCloseDelimiter(t *testing.T) {
	t.Parallel()

	content := []byte("---\nname: test\ndescription: hello\n")

	_, err := extractFrontmatter(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closing")
}

// TestParseTriggers_CommaSeparated verifies that comma-separated triggers are
// split and trimmed correctly.
func TestParseTriggers_CommaSeparated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		skillName string
		expected  []string
	}{
		{
			name:      "single trigger",
			input:     "/foo",
			skillName: "foo",
			expected:  []string{"/foo"},
		},
		{
			name:      "multiple triggers",
			input:     "/foo, /bar, /baz",
			skillName: "foo",
			expected:  []string{"/foo", "/bar", "/baz"},
		},
		{
			name:      "empty string uses skill name",
			input:     "",
			skillName: "my-skill",
			expected:  []string{"my-skill"},
		},
		{
			name:      "whitespace-only entries are dropped",
			input:     "/foo, , /bar",
			skillName: "foo",
			expected:  []string{"/foo", "/bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseTriggers(tt.input, tt.skillName)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestFindTriggerConflicts_NoConflict verifies that no conflicts are returned
// when existing skill files do not contain the same triggers.
func TestFindTriggerConflicts_NoConflict(t *testing.T) {
	t.Parallel()

	// Given: a skills directory with one skill that has different triggers
	dir := t.TempDir()
	existingContent := "---\nname: other\ntriggers:\n  - /other\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.md"), []byte(existingContent), 0o644))

	// When: findTriggerConflicts is called with non-conflicting triggers
	conflicts := findTriggerConflicts(dir, []string{"/new-trigger"})

	// Then: no conflicts
	assert.Empty(t, conflicts)
}

// TestFindTriggerConflicts_WithConflict verifies that a conflict is returned
// when an existing skill file contains the same trigger.
func TestFindTriggerConflicts_WithConflict(t *testing.T) {
	t.Parallel()

	// Given: a skills directory with a skill that uses /foo
	dir := t.TempDir()
	existingContent := "---\nname: existing\ntriggers:\n  - /foo\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.md"), []byte(existingContent), 0o644))

	// When: findTriggerConflicts is called with /foo
	conflicts := findTriggerConflicts(dir, []string{"/foo"})

	// Then: one conflict is returned
	require.Len(t, conflicts, 1)
	assert.Equal(t, "/foo", conflicts[0].trigger)
	assert.Equal(t, "existing.md", conflicts[0].file)
}

// TestFindTriggerConflicts_NonExistentDir verifies that a missing skills
// directory returns no conflicts (not an error).
func TestFindTriggerConflicts_NonExistentDir(t *testing.T) {
	t.Parallel()

	// Given: a directory that does not exist
	dir := filepath.Join(t.TempDir(), "no-such-dir")

	// When: findTriggerConflicts is called
	conflicts := findTriggerConflicts(dir, []string{"/foo"})

	// Then: empty result (no error, no conflict)
	assert.Empty(t, conflicts)
}

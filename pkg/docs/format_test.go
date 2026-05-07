package docs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormat_PromptInjection verifies that a single library doc is formatted as a prompt section.
// Given: a single DocResult with library name and content
// When: FormatPromptInjection is called
// Then: the output contains the standard "## Reference Documentation" header and library section
func TestFormat_PromptInjection(t *testing.T) {
	t.Parallel()

	result := &DocResult{
		LibraryName: "cobra",
		Source:      "context7",
		Version:     "1.9.1",
		SourceRef:   "/spf13/cobra",
		CheckedAt:   time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
		Content:     "# cobra\nCommand creation API overview.",
		Tokens:      20,
	}

	formatted, err := FormatPromptInjection([]*DocResult{result})
	require.NoError(t, err)
	assert.Contains(t, formatted, "## Reference Documentation")
	assert.Contains(t, formatted, "documentation sources")
	assert.Contains(t, formatted, "### cobra")
	assert.Contains(t, formatted, "Command creation API")
	assert.Contains(t, formatted, "(via Context7)")
	assert.Contains(t, formatted, "version=1.9.1")
	assert.Contains(t, formatted, "source_ref=/spf13/cobra")
	assert.Contains(t, formatted, "checked_at=2026-05-07")
}

// TestFormat_MultipleLibraries verifies that multiple library docs are formatted with separate headers.
// Given: two DocResults for different libraries
// When: FormatPromptInjection is called
// Then: each library appears under its own header in the correct order
func TestFormat_MultipleLibraries(t *testing.T) {
	t.Parallel()

	results := []*DocResult{
		{
			LibraryName: "cobra",
			Source:      "context7",
			Content:     "cobra documentation content",
			Tokens:      15,
		},
		{
			LibraryName: "viper",
			Source:      "scraper",
			Content:     "viper documentation content",
			Tokens:      12,
		},
	}

	formatted, err := FormatPromptInjection(results)
	require.NoError(t, err)

	assert.Contains(t, formatted, "### cobra")
	assert.Contains(t, formatted, "### viper")
	assert.Contains(t, formatted, "cobra documentation content")
	assert.Contains(t, formatted, "viper documentation content")

	// cobra must appear before viper
	cobraIdx := indexOfSubstring(formatted, "### cobra")
	viperIdx := indexOfSubstring(formatted, "### viper")
	assert.Less(t, cobraIdx, viperIdx, "cobra section must precede viper section")
}

func indexOfSubstring(s, sub string) int {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

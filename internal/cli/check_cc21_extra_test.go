package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFrontmatterContainsKey_VariousCases covers all branches.
func TestFrontmatterContainsKey_VariousCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		key     string
		want    bool
	}{
		{
			"present",
			"---\nname: foo\neffort: medium\n---\nbody",
			"effort", true,
		},
		{
			"absent",
			"---\nname: foo\n---\nbody",
			"effort", false,
		},
		{
			"no opening delimiter",
			"name: foo\neffort: medium\n",
			"effort", false,
		},
		{
			"empty content",
			"",
			"effort", false,
		},
		{
			"key is prefix of another key",
			"---\nname: foo\nefforts: max\n---\nbody",
			"effort", false,
		},
		{
			"closes before key",
			"---\n---\neffort: medium",
			"effort", false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, frontmatterContainsKey(tc.content, tc.key))
		})
	}
}

// TestCheckAgentEffort_NoDirectory returns true and skips gracefully.
func TestCheckAgentEffort_NoDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var buf bytes.Buffer
	ok := checkAgentEffort(dir, &buf, false)
	assert.True(t, ok)
	assert.Contains(t, buf.String(), "no .claude/agents/autopus")
}

// TestCheckAgentEffort_AllPresent passes when every agent declares effort.
func TestCheckAgentEffort_AllPresent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents", "autopus")
	_ = os.MkdirAll(agentsDir, 0o755)

	content := "---\nname: executor\ndescription: does things\neffort: medium\n---\n# body"
	_ = os.WriteFile(filepath.Join(agentsDir, "executor.md"), []byte(content), 0o644)

	var buf bytes.Buffer
	ok := checkAgentEffort(dir, &buf, false)
	assert.True(t, ok)
	assert.Contains(t, buf.String(), "all Claude subagents declare effort")
}

// TestCheckAgentEffort_MissingEffort fails and reports the file.
func TestCheckAgentEffort_MissingEffort(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents", "autopus")
	_ = os.MkdirAll(agentsDir, 0o755)

	bad := "---\nname: reviewer\ndescription: reviews stuff\n---\n# no effort"
	_ = os.WriteFile(filepath.Join(agentsDir, "reviewer.md"), []byte(bad), 0o644)

	var buf bytes.Buffer
	ok := checkAgentEffort(dir, &buf, false)
	assert.False(t, ok)
	assert.Contains(t, buf.String(), "missing effort field")
}

// TestCheckAgentEffort_SkipGuideFile ignores AGENT-GUIDE.md.
func TestCheckAgentEffort_SkipGuideFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents", "autopus")
	_ = os.MkdirAll(agentsDir, 0o755)

	// AGENT-GUIDE.md without effort must NOT cause a failure.
	guide := "---\nname: guide\ndescription: guide\n---\n"
	_ = os.WriteFile(filepath.Join(agentsDir, "AGENT-GUIDE.md"), []byte(guide), 0o644)

	// A valid agent alongside it.
	good := "---\nname: tester\ndescription: runs tests\neffort: high\n---\n"
	_ = os.WriteFile(filepath.Join(agentsDir, "tester.md"), []byte(good), 0o644)

	var buf bytes.Buffer
	ok := checkAgentEffort(dir, &buf, false)
	assert.True(t, ok)
}

// TestDisplayEffortValue_Stripped handles the stripped sentinel.
func TestDisplayEffortValue_Stripped(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "<stripped>", displayEffortValue(EffortStripped))
	assert.Equal(t, "max", displayEffortValue(EffortMax))
	assert.Equal(t, "medium", displayEffortValue(EffortMedium))
}
